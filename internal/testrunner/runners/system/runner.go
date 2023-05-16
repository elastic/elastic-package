// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

const (
	testRunMaxID = 99999
	testRunMinID = 10000
)

func init() {
	testrunner.RegisterRunner(&runner{})
}

const (
	// TestType defining system tests
	TestType testrunner.TestType = "system"

	// Maximum number of events to query.
	elasticsearchQuerySize = 500

	// ServiceLogsAgentDir is folder path where log files produced by the service
	// are stored on the Agent container's filesystem.
	ServiceLogsAgentDir = "/tmp/service_logs"

	waitForDataDefaultTimeout = 10 * time.Minute
)

type logsRegexp struct {
	includes *regexp.Regexp
	excludes []*regexp.Regexp
}

type logsByContainer struct {
	containerName string
	patterns      []logsRegexp
}

var (
	errorPatterns = []logsByContainer{
		logsByContainer{
			containerName: "elastic-agent",
			patterns: []logsRegexp{
				logsRegexp{
					includes: regexp.MustCompile("^Cannot index event publisher.Event"),
					excludes: []*regexp.Regexp{
						// this regex is excluded to ensure that logs coming from the `system` package installed by default are not taken into account
						regexp.MustCompile(`action \[indices:data\/write\/bulk\[s\]\] is unauthorized for API key id \[.*\] of user \[.*\] on indices \[.*\], this action is granted by the index privileges \[.*\]`),
					},
				},
				logsRegexp{
					includes: regexp.MustCompile("->(FAILED|DEGRADED)"),
				},
			},
		},
	}
)

type runner struct {
	options testrunner.TestOptions

	// Execution order of following handlers is defined in runner.TearDown() method.
	deleteTestPolicyHandler func() error
	resetAgentPolicyHandler func() error
	shutdownServiceHandler  func() error
	wipeDataStreamHandler   func() error
}

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

// String returns the human-friendly name of the test runner.
func (r *runner) String() string {
	return "system"
}

// CanRunPerDataStream returns whether this test runner can run on individual
// data streams within the package.
func (r *runner) CanRunPerDataStream() bool {
	return true
}

func (r *runner) TestFolderRequired() bool {
	return true
}

// Run runs the system tests defined under the given folder
func (r *runner) Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.options = options
	return r.run()
}

// TearDown method doesn't perform any global action as the "tear down" is executed per test case.
func (r *runner) TearDown() error {
	return nil
}

func (r *runner) tearDownTest() error {
	if r.options.DeferCleanup > 0 {
		logger.Debugf("waiting for %s before tearing down...", r.options.DeferCleanup)
		signal.Sleep(r.options.DeferCleanup)
	}

	if r.resetAgentPolicyHandler != nil {
		if err := r.resetAgentPolicyHandler(); err != nil {
			return err
		}
		r.resetAgentPolicyHandler = nil
	}

	if r.deleteTestPolicyHandler != nil {
		if err := r.deleteTestPolicyHandler(); err != nil {
			return err
		}
		r.deleteTestPolicyHandler = nil
	}

	if r.shutdownServiceHandler != nil {
		if err := r.shutdownServiceHandler(); err != nil {
			return err
		}
		r.shutdownServiceHandler = nil
	}

	if r.wipeDataStreamHandler != nil {
		if err := r.wipeDataStreamHandler(); err != nil {
			return err
		}
		r.wipeDataStreamHandler = nil
	}

	return nil
}

func (r *runner) newResult(name string) *testrunner.ResultComposer {
	return testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   TestType,
		Name:       name,
		Package:    r.options.TestFolder.Package,
		DataStream: r.options.TestFolder.DataStream,
	})
}

func (r *runner) run() (results []testrunner.TestResult, err error) {
	result := r.newResult("(init)")
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return result.WithError(errors.Wrap(err, "reading service logs directory failed"))
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return result.WithError(errors.Wrap(err, "locating data stream root failed"))
	}
	if found {
		logger.Debug("Running system tests for data stream")
	} else {
		logger.Debug("Running system tests for package")
	}

	devDeployPath, err := servicedeployer.FindDevDeployPath(servicedeployer.FactoryOptions{
		PackageRootPath:    r.options.PackageRootPath,
		DataStreamRootPath: dataStreamPath,
	})
	if err != nil {
		return result.WithError(errors.Wrap(err, "_dev/deploy directory not found"))
	}

	cfgFiles, err := listConfigFiles(r.options.TestFolder.Path)
	if err != nil {
		return result.WithError(errors.Wrap(err, "failed listing test case config cfgFiles"))
	}

	variantsFile, err := servicedeployer.ReadVariantsFile(devDeployPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result.WithError(errors.Wrap(err, "can't read service variant"))
	}

	startTesting := time.Now()
	for _, cfgFile := range cfgFiles {
		for _, variantName := range r.selectVariants(variantsFile) {
			partial, err := r.runTestPerVariant(result, locationManager, cfgFile, dataStreamPath, variantName)
			results = append(results, partial...)
			if err != nil {
				return results, err
			}
		}
	}

	tempDir, err := os.MkdirTemp("", "test-system-")
	if err != nil {
		return nil, fmt.Errorf("can't create temporal directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	dumpOptions := stack.DumpOptions{Output: tempDir, Profile: r.options.Profile}
	_, err = stack.Dump(dumpOptions)
	if err != nil {
		return nil, fmt.Errorf("dump failed: %w", err)
	}

	logResults, err := r.checkAgentLogs(dumpOptions, startTesting, errorPatterns)
	if err != nil {
		return result.WithError(err)
	}
	results = append(results, logResults...)

	return results, nil
}

func (r *runner) runTestPerVariant(result *testrunner.ResultComposer, locationManager *locations.LocationManager, cfgFile, dataStreamPath, variantName string) ([]testrunner.TestResult, error) {
	serviceOptions := servicedeployer.FactoryOptions{
		PackageRootPath:    r.options.PackageRootPath,
		DataStreamRootPath: dataStreamPath,
		Variant:            variantName,
	}

	var ctxt servicedeployer.ServiceContext
	ctxt.Name = r.options.TestFolder.Package
	ctxt.Logs.Folder.Local = locationManager.ServiceLogDir()
	ctxt.Logs.Folder.Agent = ServiceLogsAgentDir
	ctxt.Test.RunID = createTestRunID()
	testConfig, err := newConfig(filepath.Join(r.options.TestFolder.Path, cfgFile), ctxt, variantName)
	if err != nil {
		return result.WithError(errors.Wrapf(err, "unable to load system test case file '%s'", cfgFile))
	}

	var partial []testrunner.TestResult
	if testConfig.Skip == nil {
		logger.Debugf("running test with configuration '%s'", testConfig.Name())
		partial, err = r.runTest(testConfig, ctxt, serviceOptions)
	} else {
		logger.Warnf("skipping %s test for %s/%s: %s (details: %s)",
			TestType, r.options.TestFolder.Package, r.options.TestFolder.DataStream,
			testConfig.Skip.Reason, testConfig.Skip.Link.String())
		result := r.newResult(testConfig.Name())
		partial, err = result.WithSkip(testConfig.Skip)
	}

	tdErr := r.tearDownTest()
	if err != nil {
		return partial, err
	}
	if tdErr != nil {
		return partial, errors.Wrap(tdErr, "failed to tear down runner")
	}
	return partial, nil
}

func createTestRunID() string {
	return fmt.Sprintf("%d", rand.Intn(testRunMaxID-testRunMinID)+testRunMinID)
}

func (r *runner) getDocs(dataStream string) ([]common.MapStr, error) {
	resp, err := r.options.API.Search(
		r.options.API.Search.WithIndex(dataStream),
		r.options.API.Search.WithSort("@timestamp:asc"),
		r.options.API.Search.WithSize(elasticsearchQuerySize),
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not search data stream")
	}
	defer resp.Body.Close()

	var results struct {
		Hits struct {
			Total struct {
				Value int
			}
			Hits []struct {
				Source common.MapStr `json:"_source"`
			}
		}
		Error *struct {
			Type   string
			Reason string
		}
		Status int
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, errors.Wrap(err, "could not decode search results response")
	}

	numHits := results.Hits.Total.Value
	if results.Error != nil {
		logger.Debugf("found %d hits in %s data stream: %s: %s Status=%d",
			numHits, dataStream, results.Error.Type, results.Error.Reason, results.Status)
	} else {
		logger.Debugf("found %d hits in %s data stream", numHits, dataStream)
	}

	var docs []common.MapStr
	for _, hit := range results.Hits.Hits {
		docs = append(docs, hit.Source)
	}

	return docs, nil
}

func (r *runner) runTest(config *testConfig, ctxt servicedeployer.ServiceContext, serviceOptions servicedeployer.FactoryOptions) ([]testrunner.TestResult, error) {
	result := r.newResult(config.Name())

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return result.WithError(errors.Wrap(err, "reading package manifest failed"))
	}

	dataStreamManifest, err := packages.ReadDataStreamManifest(filepath.Join(serviceOptions.DataStreamRootPath, packages.DataStreamManifestFile))
	if err != nil {
		return result.WithError(errors.Wrap(err, "reading data stream manifest failed"))
	}

	policyTemplateName := config.PolicyTemplate
	if policyTemplateName == "" {
		policyTemplateName, err = findPolicyTemplateForInput(*pkgManifest, *dataStreamManifest, config.Input)
		if err != nil {
			return result.WithError(errors.Wrap(err, "failed to determine the associated policy_template"))
		}
	}

	policyTemplate, err := selectPolicyTemplateByName(pkgManifest.PolicyTemplates, policyTemplateName)
	if err != nil {
		return result.WithError(errors.Wrap(err, "failed to find the selected policy_template"))
	}

	// Setup service.
	logger.Debug("setting up service...")
	serviceDeployer, err := servicedeployer.Factory(serviceOptions)
	if err != nil {
		return result.WithError(errors.Wrap(err, "could not create service runner"))
	}

	if config.Service != "" {
		ctxt.Name = config.Service
	}
	service, err := serviceDeployer.SetUp(ctxt)
	if err != nil {
		return result.WithError(errors.Wrap(err, "could not setup service"))
	}
	ctxt = service.Context()
	r.shutdownServiceHandler = func() error {
		logger.Debug("tearing down service...")
		if err := service.TearDown(); err != nil {
			return errors.Wrap(err, "error tearing down service")
		}

		return nil
	}

	// Reload test config with ctx variable substitution.
	config, err = newConfig(config.Path, ctxt, serviceOptions.Variant)
	if err != nil {
		return result.WithError(errors.Wrap(err, "unable to reload system test case configuration"))
	}

	kib, err := kibana.NewClient()
	if err != nil {
		return result.WithError(errors.Wrap(err, "can't create Kibana client"))
	}

	// Configure package (single data stream) via Ingest Manager APIs.
	logger.Debug("creating test policy...")
	testTime := time.Now().Format("20060102T15:04:05Z")
	p := kibana.Policy{
		Name:        fmt.Sprintf("ep-test-system-%s-%s-%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream, testTime),
		Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream),
		Namespace:   "ep",
	}
	policy, err := kib.CreatePolicy(p)
	if err != nil {
		return result.WithError(errors.Wrap(err, "could not create test policy"))
	}
	r.deleteTestPolicyHandler = func() error {
		logger.Debug("deleting test policy...")
		if err := kib.DeletePolicy(*policy); err != nil {
			return errors.Wrap(err, "error cleaning up test policy")
		}
		return nil
	}

	logger.Debug("adding package data stream to test policy...")
	ds := createPackageDatastream(*policy, *pkgManifest, policyTemplate, *dataStreamManifest, *config)
	if err := kib.AddPackageDataStreamToPolicy(ds); err != nil {
		return result.WithError(errors.Wrap(err, "could not add data stream config to policy"))
	}

	// Delete old data
	logger.Debug("deleting old data in data stream...")
	dataStream := fmt.Sprintf(
		"%s-%s-%s",
		ds.Inputs[0].Streams[0].DataStream.Type,
		ds.Inputs[0].Streams[0].DataStream.Dataset,
		ds.Namespace,
	)

	r.wipeDataStreamHandler = func() error {
		logger.Debugf("deleting data in data stream...")
		if err := deleteDataStreamDocs(r.options.API, dataStream); err != nil {
			return errors.Wrap(err, "error deleting data in data stream")
		}
		return nil
	}

	if err := deleteDataStreamDocs(r.options.API, dataStream); err != nil {
		return result.WithError(errors.Wrapf(err, "error deleting old data in data stream: %s", dataStream))
	}

	cleared, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return true, errors.New("SIGINT: cancel clearing data")
		}

		docs, err := r.getDocs(dataStream)
		return len(docs) == 0, err
	}, 2*time.Minute)
	if err != nil || !cleared {
		if err == nil {
			err = errors.New("unable to clear previous data")
		}
		return result.WithError(err)
	}

	agents, err := checkEnrolledAgents(kib, ctxt)
	if err != nil {
		return result.WithError(errors.Wrap(err, "can't check enrolled agents"))
	}
	agent := agents[0]
	origPolicy := kibana.Policy{
		ID:       agent.PolicyID,
		Revision: agent.PolicyRevision,
	}

	// Assign policy to agent
	r.resetAgentPolicyHandler = func() error {
		logger.Debug("reassigning original policy back to agent...")
		if err := kib.AssignPolicyToAgent(agent, origPolicy); err != nil {
			return errors.Wrap(err, "error reassigning original policy to agent")
		}
		return nil
	}

	policyWithDataStream, err := kib.GetPolicy(policy.ID)
	if err != nil {
		return result.WithError(errors.Wrap(err, "could not read the policy with data stream"))
	}

	logger.Debug("assigning package data stream to agent...")
	if err := kib.AssignPolicyToAgent(agent, *policyWithDataStream); err != nil {
		return result.WithError(errors.Wrap(err, "could not assign policy to agent"))
	}

	// Signal to the service that the agent is ready (policy is assigned).
	if config.ServiceNotifySignal != "" {
		if err = service.Signal(config.ServiceNotifySignal); err != nil {
			return result.WithError(errors.Wrap(err, "failed to notify test service"))
		}
	}

	// Use custom timeout if the service can't collect data immediately.
	waitForDataTimeout := waitForDataDefaultTimeout
	if config.WaitForDataTimeout > 0 {
		waitForDataTimeout = config.WaitForDataTimeout
	}

	// (TODO in future) Optionally exercise service to generate load.
	logger.Debug("checking for expected data in data stream...")
	var docs []common.MapStr
	passed, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return true, errors.New("SIGINT: cancel waiting for policy assigned")
		}

		var err error
		docs, err = r.getDocs(dataStream)

		if config.Assert.HitCount > 0 {
			return len(docs) >= config.Assert.HitCount, err
		}

		return len(docs) > 0, err
	}, waitForDataTimeout)
	if err != nil {
		return result.WithError(err)
	}

	if !passed {
		result.FailureMsg = fmt.Sprintf("could not find hits in %s data stream", dataStream)
		return result.WithError(fmt.Errorf("%s", result.FailureMsg))
	}

	// Validate fields in docs
	var expectedDataset string
	if ds := r.options.TestFolder.DataStream; ds != "" {
		expectedDataset = getDataStreamDataset(*pkgManifest, *dataStreamManifest)
	} else {
		expectedDataset = pkgManifest.Name + "." + policyTemplateName
	}
	fieldsValidator, err := fields.CreateValidatorForDirectory(serviceOptions.DataStreamRootPath,
		fields.WithSpecVersion(pkgManifest.SpecVersion),
		fields.WithNumericKeywordFields(config.NumericKeywordFields),
		fields.WithExpectedDataset(expectedDataset),
		fields.WithEnabledImportAllECSSChema(true),
	)
	if err != nil {
		return result.WithError(errors.Wrapf(err, "creating fields validator for data stream failed (path: %s)", serviceOptions.DataStreamRootPath))
	}
	if err := validateFields(docs, fieldsValidator, dataStream); err != nil {
		return result.WithError(err)
	}

	// Write sample events file from first doc, if requested
	if err := r.generateTestResult(docs); err != nil {
		return result.WithError(err)
	}

	// Check Hit Count within docs, if 0 then it has not been specified
	if assertionPass, message := assertHitCount(config.Assert.HitCount, docs); !assertionPass {
		result.FailureMsg = message
	}

	return result.WithSuccess()
}

func checkEnrolledAgents(client *kibana.Client, ctxt servicedeployer.ServiceContext) ([]kibana.Agent, error) {
	var agents []kibana.Agent
	enrolled, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return false, errors.New("SIGINT: cancel checking enrolled agents")
		}

		allAgents, err := client.ListAgents()
		if err != nil {
			return false, errors.Wrap(err, "could not list agents")
		}

		agents = filterAgents(allAgents, ctxt)
		logger.Debugf("found %d enrolled agent(s)", len(agents))
		if len(agents) == 0 {
			return false, nil // selected agents are unavailable yet
		}
		return true, nil
	}, 5*time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "agent enrollment failed")
	}
	if !enrolled {
		return nil, errors.New("no agent enrolled in time")
	}
	return agents, nil
}

func createPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	ds packages.DataStreamManifest,
	config testConfig,
) kibana.PackageDataStream {
	if pkg.Type == "input" {
		return createInputPackageDatastream(kibanaPolicy, pkg, policyTemplate, config)
	}
	return createIntegrationPackageDatastream(kibanaPolicy, pkg, policyTemplate, ds, config)
}

func createIntegrationPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	ds packages.DataStreamManifest,
	config testConfig,
) kibana.PackageDataStream {
	r := kibana.PackageDataStream{
		Name:      fmt.Sprintf("%s-%s", pkg.Name, ds.Name),
		Namespace: "ep",
		PolicyID:  kibanaPolicy.ID,
		Enabled:   true,
		Inputs: []kibana.Input{
			{
				PolicyTemplate: policyTemplate.Name,
				Enabled:        true,
			},
		},
	}
	r.Package.Name = pkg.Name
	r.Package.Title = pkg.Title
	r.Package.Version = pkg.Version

	stream := ds.Streams[getDataStreamIndex(config.Input, ds)]
	streamInput := stream.Input
	r.Inputs[0].Type = streamInput

	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, ds.Name),
			Enabled: true,
			DataStream: kibana.DataStream{
				Type:    ds.Type,
				Dataset: getDataStreamDataset(pkg, ds),
			},
		},
	}

	// Add dataStream-level vars
	streams[0].Vars = setKibanaVariables(stream.Vars, config.DataStream.Vars)
	r.Inputs[0].Streams = streams

	// Add package-level vars
	var inputVars []packages.Variable
	input := policyTemplate.FindInputByType(streamInput)
	if input != nil {
		// copy package-level vars into each input
		inputVars = append(inputVars, input.Vars...)
		inputVars = append(inputVars, pkg.Vars...)
	}

	r.Inputs[0].Vars = setKibanaVariables(inputVars, config.Vars)

	return r
}

func createInputPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	config testConfig,
) kibana.PackageDataStream {
	r := kibana.PackageDataStream{
		Name:      fmt.Sprintf("%s-%s", pkg.Name, policyTemplate.Name),
		Namespace: "ep",
		PolicyID:  kibanaPolicy.ID,
		Enabled:   true,
	}
	r.Package.Name = pkg.Name
	r.Package.Title = pkg.Title
	r.Package.Version = pkg.Version
	r.Inputs = []kibana.Input{
		{
			PolicyTemplate: policyTemplate.Name,
			Enabled:        true,
			Vars:           kibana.Vars{},
		},
	}

	streamInput := policyTemplate.Input
	r.Inputs[0].Type = streamInput

	dataset := fmt.Sprintf("%s.%s", pkg.Name, policyTemplate.Name)
	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, policyTemplate.Name),
			Enabled: true,
			DataStream: kibana.DataStream{
				Type:    policyTemplate.Type,
				Dataset: dataset,
			},
		},
	}

	// Add policyTemplate-level vars.
	vars := setKibanaVariables(policyTemplate.Vars, config.Vars)
	if _, found := vars["data_stream.dataset"]; !found {
		var value packages.VarValue
		value.Unpack(dataset)
		vars["data_stream.dataset"] = kibana.Var{
			Value: value,
			Type:  "text",
		}
	}

	streams[0].Vars = vars
	r.Inputs[0].Streams = streams
	return r
}

func setKibanaVariables(definitions []packages.Variable, values common.MapStr) kibana.Vars {
	vars := kibana.Vars{}
	for _, definition := range definitions {
		val := definition.Default

		value, err := values.GetValue(definition.Name)
		if err == nil {
			val = packages.VarValue{}
			val.Unpack(value)
		}

		vars[definition.Name] = kibana.Var{
			Type:  definition.Type,
			Value: val,
		}
	}
	return vars
}

// getDataStreamIndex returns the index of the data stream whose input name
// matches. Otherwise it returns the 0.
func getDataStreamIndex(inputName string, ds packages.DataStreamManifest) int {
	for i, s := range ds.Streams {
		if s.Input == inputName {
			return i
		}
	}
	return 0
}

func getDataStreamDataset(pkg packages.PackageManifest, ds packages.DataStreamManifest) string {
	if len(ds.Dataset) > 0 {
		return ds.Dataset
	}
	return fmt.Sprintf("%s.%s", pkg.Name, ds.Name)
}

// findPolicyTemplateForInput returns the name of the policy_template that
// applies to the input under test. An error is returned if no policy template
// matches or if multiple policy templates match and the response is ambiguous.
func findPolicyTemplateForInput(pkg packages.PackageManifest, ds packages.DataStreamManifest, inputName string) (string, error) {
	if pkg.Type == "input" {
		return findPolicyTemplateForInputPackage(pkg, inputName)
	}
	return findPolicyTemplateForDataStream(pkg, ds, inputName)
}

func findPolicyTemplateForDataStream(pkg packages.PackageManifest, ds packages.DataStreamManifest, inputName string) (string, error) {
	if inputName == "" {
		if len(ds.Streams) == 0 {
			return "", errors.New("no streams declared in data stream manifest")
		}
		inputName = ds.Streams[getDataStreamIndex(inputName, ds)].Input
	}

	var matchedPolicyTemplates []string
	for _, policyTemplate := range pkg.PolicyTemplates {
		// Does this policy_template include this input type?
		if policyTemplate.FindInputByType(inputName) == nil {
			continue
		}

		// Does the policy_template apply to this data stream (when data streams are specified)?
		if len(policyTemplate.DataStreams) > 0 && !common.StringSliceContains(policyTemplate.DataStreams, ds.Name) {
			continue
		}

		matchedPolicyTemplates = append(matchedPolicyTemplates, policyTemplate.Name)
	}

	switch len(matchedPolicyTemplates) {
	case 1:
		return matchedPolicyTemplates[0], nil
	case 0:
		return "", fmt.Errorf("no policy template was found for data stream %q "+
			"with input type %q: verify that you have included the data stream "+
			"and input in the package's policy_template list", ds.Name, inputName)
	default:
		return "", fmt.Errorf("ambiguous result: multiple policy templates ([%s]) "+
			"were found that apply to data stream %q with input type %q: please "+
			"specify the 'policy_template' in the system test config",
			strings.Join(matchedPolicyTemplates, ", "), ds.Name, inputName)
	}
}

func findPolicyTemplateForInputPackage(pkg packages.PackageManifest, inputName string) (string, error) {
	if inputName == "" {
		if len(pkg.PolicyTemplates) == 0 {
			return "", errors.New("no policy templates specified for input package")
		}
		inputName = pkg.PolicyTemplates[0].Input
	}

	var matched []string
	for _, policyTemplate := range pkg.PolicyTemplates {
		if policyTemplate.Input != inputName {
			continue
		}

		matched = append(matched, policyTemplate.Name)
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", fmt.Errorf("no policy template was found"+
			"with input type %q: verify that you have included the data stream "+
			"and input in the package's policy_template list", inputName)
	default:
		return "", fmt.Errorf("ambiguous result: multiple policy templates ([%s]) "+
			"with input type %q: please "+
			"specify the 'policy_template' in the system test config",
			strings.Join(matched, ", "), inputName)
	}
}

func selectPolicyTemplateByName(policies []packages.PolicyTemplate, name string) (packages.PolicyTemplate, error) {
	for _, policy := range policies {
		if policy.Name == name {
			return policy, nil
		}
	}
	return packages.PolicyTemplate{}, fmt.Errorf("policy template %q not found", name)
}

func deleteDataStreamDocs(api *elasticsearch.API, dataStream string) error {
	body := strings.NewReader(`{ "query": { "match_all": {} } }`)
	_, err := api.DeleteByQuery([]string{dataStream}, body)
	if err != nil {
		return err
	}

	return nil
}

func waitUntilTrue(fn func() (bool, error), timeout time.Duration) (bool, error) {
	timeoutTicker := time.NewTicker(timeout)
	defer timeoutTicker.Stop()

	retryTicker := time.NewTicker(1 * time.Second)
	defer retryTicker.Stop()

	for {
		result, err := fn()
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}

		select {
		case <-retryTicker.C:
			continue
		case <-timeoutTicker.C:
			return false, nil
		}
	}
}

func filterAgents(allAgents []kibana.Agent, ctx servicedeployer.ServiceContext) []kibana.Agent {
	if ctx.Agent.Host.NamePrefix != "" {
		logger.Debugf("filter agents using criteria: NamePrefix=%s", ctx.Agent.Host.NamePrefix)
	}

	var filtered []kibana.Agent
	for _, agent := range allAgents {
		if agent.PolicyRevision == 0 {
			continue // For some reason Kibana doesn't always return a valid policy revision (eventually it will be present and valid)
		}

		if ctx.Agent.Host.NamePrefix != "" && !strings.HasPrefix(agent.LocalMetadata.Host.Name, ctx.Agent.Host.NamePrefix) {
			continue
		}
		filtered = append(filtered, agent)
	}
	return filtered
}

func writeSampleEvent(path string, doc common.MapStr) error {
	body, err := json.MarshalIndent(doc, "", "    ")
	if err != nil {
		return errors.Wrap(err, "marshalling sample event failed")
	}

	err = os.WriteFile(filepath.Join(path, "sample_event.json"), body, 0644)
	if err != nil {
		return errors.Wrap(err, "writing sample event failed")
	}

	return nil
}

func validateFields(docs []common.MapStr, fieldsValidator *fields.Validator, dataStream string) error {
	var multiErr multierror.Error
	for _, doc := range docs {
		if message, err := doc.GetValue("error.message"); err != common.ErrKeyNotFound {
			multiErr = append(multiErr, fmt.Errorf("found error.message in event: %v", message))
			continue
		}

		errs := fieldsValidator.ValidateDocumentMap(doc)
		if errs != nil {
			multiErr = append(multiErr, errs...)
			continue
		}
	}

	if len(multiErr) > 0 {
		multiErr = multiErr.Unique()
		return testrunner.ErrTestCaseFailed{
			Reason:  fmt.Sprintf("one or more errors found in documents stored in %s data stream", dataStream),
			Details: multiErr.Error(),
		}
	}

	return nil
}

func assertHitCount(expected int, docs []common.MapStr) (pass bool, message string) {
	if expected != 0 {
		observed := len(docs)
		logger.Debugf("assert hit count expected %d, observed %d", expected, observed)
		if observed != expected {
			return false, fmt.Sprintf("observed hit count %d did not match expected hit count %d", observed, expected)
		}
	}
	return true, ""
}

func (r *runner) selectVariants(variantsFile *servicedeployer.VariantsFile) []string {
	if variantsFile == nil || variantsFile.Variants == nil {
		return []string{""} // empty variants file switches to no-variant mode
	}

	var variantNames []string
	for k := range variantsFile.Variants {
		if r.options.ServiceVariant != "" && r.options.ServiceVariant != k {
			continue
		}
		variantNames = append(variantNames, k)
	}
	return variantNames
}

func (r *runner) generateTestResult(docs []common.MapStr) error {
	if !r.options.GenerateTestResult {
		return nil
	}

	rootPath := r.options.PackageRootPath
	if ds := r.options.TestFolder.DataStream; ds != "" {
		rootPath = filepath.Join(rootPath, "data_stream", ds)
	}

	if err := writeSampleEvent(rootPath, docs[0]); err != nil {
		return errors.Wrap(err, "failed to write sample event file")
	}

	return nil
}

func (r *runner) checkAgentLogs(dumpOptions stack.DumpOptions, startTesting time.Time, errorPatterns []logsByContainer) (results []testrunner.TestResult, err error) {

	for _, patternsContainer := range errorPatterns {
		startTime := time.Now()

		serviceLogsFile := stack.DumpLogsFile(dumpOptions, patternsContainer.containerName)

		err = r.anyErrorMessages(serviceLogsFile, startTesting, patternsContainer.patterns)
		if e, ok := err.(testrunner.ErrTestCaseFailed); ok {
			tr := testrunner.TestResult{
				TestType:   TestType,
				Name:       fmt.Sprintf("(%s logs)", patternsContainer.containerName),
				Package:    r.options.TestFolder.Package,
				DataStream: r.options.TestFolder.DataStream,
			}
			tr.FailureMsg = e.Error()
			tr.FailureDetails = e.Details
			tr.TimeElapsed = time.Since(startTime)
			results = append(results, tr)
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("check log messages failed: %s", err)
		}
	}
	return results, nil
}

func (r *runner) anyErrorMessages(logsFilePath string, startTime time.Time, errorPatterns []logsRegexp) error {
	var multiErr multierror.Error
	processLog := func(log stack.LogLine) error {
		for _, pattern := range errorPatterns {
			if !pattern.includes.MatchString(log.Message) {
				continue
			}
			isExcluded := false
			for _, excludes := range pattern.excludes {
				if excludes.MatchString(log.Message) {
					isExcluded = true
					break
				}
			}
			if isExcluded {
				continue
			}

			multiErr = append(multiErr, fmt.Errorf("found error %q", log.Message))
		}
		return nil
	}
	err := stack.ParseLogs(stack.ParseLogsOptions{
		LogsFilePath: logsFilePath,
		StartTime:    startTime,
	}, processLog)
	if err != nil {
		return err
	}

	if len(multiErr) > 0 {
		return testrunner.ErrTestCaseFailed{
			Reason:  fmt.Sprintf("one or more errors found while examining %s", filepath.Base(logsFilePath)),
			Details: multiErr.Error(),
		}
	}
	return nil
}
