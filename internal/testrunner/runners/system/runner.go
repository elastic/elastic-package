// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"
	"github.com/elastic/elastic-package/internal/signal"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

const (
	testRunMaxID = 99999
	testRunMinID = 10000

	allFieldsBody = `{"fields": ["*"]}`
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
	options   testrunner.TestOptions
	pipelines []ingest.Pipeline
	// Execution order of following handlers is defined in runner.TearDown() method.
	deleteTestPolicyHandler func() error
	deletePackageHandler    func() error
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

	if r.deletePackageHandler != nil {
		if err := r.deletePackageHandler(); err != nil {
			return err
		}
		r.deletePackageHandler = nil
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
		return result.WithError(fmt.Errorf("reading service logs directory failed: %w", err))
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return result.WithError(fmt.Errorf("locating data stream root failed: %w", err))
	}
	if found {
		logger.Debug("Running system tests for data stream")
	} else {
		logger.Debug("Running system tests for package")
	}

	devDeployPath, err := servicedeployer.FindDevDeployPath(servicedeployer.FactoryOptions{
		Profile:            r.options.Profile,
		PackageRootPath:    r.options.PackageRootPath,
		DataStreamRootPath: dataStreamPath,
	})
	if err != nil {
		return result.WithError(fmt.Errorf("_dev/deploy directory not found: %w", err))
	}

	cfgFiles, err := listConfigFiles(r.options.TestFolder.Path)
	if err != nil {
		return result.WithError(fmt.Errorf("failed listing test case config cfgFiles: %w", err))
	}

	variantsFile, err := servicedeployer.ReadVariantsFile(devDeployPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result.WithError(fmt.Errorf("can't read service variant: %w", err))
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
		Profile:            r.options.Profile,
		PackageRootPath:    r.options.PackageRootPath,
		DataStreamRootPath: dataStreamPath,
		Variant:            variantName,
	}

	var ctxt servicedeployer.ServiceContext
	ctxt.Name = r.options.TestFolder.Package
	ctxt.Logs.Folder.Local = locationManager.ServiceLogDir()
	ctxt.Logs.Folder.Agent = ServiceLogsAgentDir
	ctxt.Test.RunID = createTestRunID()

	outputDir, err := createOutputDir(locationManager, ctxt.Test.RunID)
	if err != nil {
		return nil, fmt.Errorf("could not create output dir for terraform deployer %w", err)
	}
	ctxt.OutputDir = outputDir

	testConfig, err := newConfig(filepath.Join(r.options.TestFolder.Path, cfgFile), ctxt, variantName)
	if err != nil {
		return result.WithError(fmt.Errorf("unable to load system test case file '%s': %w", cfgFile, err))
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
		return partial, fmt.Errorf("failed to tear down runner: %w", tdErr)
	}
	return partial, nil
}

func createOutputDir(locationManager *locations.LocationManager, runId string) (string, error) {
	outputDir := filepath.Join(locationManager.ServiceOutputDir(), runId)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}
	return outputDir, nil
}

func createTestRunID() string {
	return fmt.Sprintf("%d", rand.Intn(testRunMaxID-testRunMinID)+testRunMinID)
}

func (r *runner) isSyntheticsEnabled(dataStream, componentTemplatePackage string) (bool, error) {
	logger.Debugf("check whether or not synthetics is enabled (component template %s)...", componentTemplatePackage)
	resp, err := r.options.API.Cluster.GetComponentTemplate(
		r.options.API.Cluster.GetComponentTemplate.WithName(componentTemplatePackage),
	)
	if err != nil {
		return false, fmt.Errorf("could not get component template from data stream %s: %w", dataStream, err)
	}
	defer resp.Body.Close()

	var results struct {
		ComponentTemplates []struct {
			Name              string `json:"name"`
			ComponentTemplate struct {
				Template struct {
					Mappings struct {
						Source *struct {
							Mode string `json:"mode"`
						} `json:"_source,omitempty"`
					} `json:"mappings"`
				} `json:"template"`
			} `json:"component_template"`
		} `json:"component_templates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return false, fmt.Errorf("could not decode search results response: %w", err)
	}

	if len(results.ComponentTemplates) == 0 {
		logger.Debugf("no component template found for data stream %s", dataStream)
		return false, nil
	}
	if len(results.ComponentTemplates) != 1 {
		return false, fmt.Errorf("unexpected response, not found component template")
	}

	template := results.ComponentTemplates[0]

	if template.ComponentTemplate.Template.Mappings.Source == nil {
		return false, nil
	}

	return template.ComponentTemplate.Template.Mappings.Source.Mode == "synthetic", nil
}

type hits struct {
	Source []common.MapStr `json:"_source"`
	Fields []common.MapStr `json:"fields"`
}

func (h hits) getDocs(syntheticsEnabled bool) []common.MapStr {
	if syntheticsEnabled {
		return h.Fields
	}
	return h.Source
}

func (h hits) size() int {
	return len(h.Source)
}

func (r *runner) getDocs(dataStream string) (*hits, error) {
	resp, err := r.options.API.Search(
		r.options.API.Search.WithIndex(dataStream),
		r.options.API.Search.WithSort("@timestamp:asc"),
		r.options.API.Search.WithSize(elasticsearchQuerySize),
		r.options.API.Search.WithSource("true"),
		r.options.API.Search.WithBody(strings.NewReader(allFieldsBody)),
	)
	if err != nil {
		return nil, fmt.Errorf("could not search data stream: %w", err)
	}
	defer resp.Body.Close()

	var results struct {
		Hits struct {
			Total struct {
				Value int
			}
			Hits []struct {
				Source common.MapStr `json:"_source"`
				Fields common.MapStr `json:"fields"`
			}
		}
		Error *struct {
			Type   string
			Reason string
		}
		Status int
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("could not decode search results response: %w", err)
	}

	numHits := results.Hits.Total.Value
	if results.Error != nil {
		logger.Debugf("found %d hits in %s data stream: %s: %s Status=%d",
			numHits, dataStream, results.Error.Type, results.Error.Reason, results.Status)
	} else {
		logger.Debugf("found %d hits in %s data stream", numHits, dataStream)
	}

	var hits hits
	for _, hit := range results.Hits.Hits {
		hits.Source = append(hits.Source, hit.Source)
		hits.Fields = append(hits.Fields, hit.Fields)
	}

	return &hits, nil
}

func (r *runner) runTest(config *testConfig, ctxt servicedeployer.ServiceContext, serviceOptions servicedeployer.FactoryOptions) ([]testrunner.TestResult, error) {
	result := r.newResult(config.Name())

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return result.WithError(fmt.Errorf("reading package manifest failed: %w", err))
	}

	dataStreamManifest, err := packages.ReadDataStreamManifest(filepath.Join(serviceOptions.DataStreamRootPath, packages.DataStreamManifestFile))
	if err != nil {
		return result.WithError(fmt.Errorf("reading data stream manifest failed: %w", err))
	}

	policyTemplateName := config.PolicyTemplate
	if policyTemplateName == "" {
		policyTemplateName, err = findPolicyTemplateForInput(*pkgManifest, *dataStreamManifest, config.Input)
		if err != nil {
			return result.WithError(fmt.Errorf("failed to determine the associated policy_template: %w", err))
		}
	}

	policyTemplate, err := selectPolicyTemplateByName(pkgManifest.PolicyTemplates, policyTemplateName)
	if err != nil {
		return result.WithError(fmt.Errorf("failed to find the selected policy_template: %w", err))
	}

	// Setup service.
	logger.Debug("setting up service...")
	serviceDeployer, err := servicedeployer.Factory(serviceOptions)
	if err != nil {
		return result.WithError(fmt.Errorf("could not create service runner: %w", err))
	}

	if config.Service != "" {
		ctxt.Name = config.Service
	}
	service, err := serviceDeployer.SetUp(ctxt)
	if err != nil {
		return result.WithError(fmt.Errorf("could not setup service: %w", err))
	}
	ctxt = service.Context()
	r.shutdownServiceHandler = func() error {
		logger.Debug("tearing down service...")
		if err := service.TearDown(); err != nil {
			return fmt.Errorf("error tearing down service: %w", err)
		}

		return nil
	}

	// Reload test config with ctx variable substitution.
	config, err = newConfig(config.Path, ctxt, serviceOptions.Variant)
	if err != nil {
		return result.WithError(fmt.Errorf("unable to reload system test case configuration: %w", err))
	}

	kib, err := kibana.NewClient()
	if err != nil {
		return result.WithError(fmt.Errorf("can't create Kibana client: %w", err))
	}

	// Install the package before creating the policy, so we control exactly what is being
	// installed.
	logger.Debug("Installing package...")
	installer, err := installer.NewForPackage(installer.Options{
		Kibana:         kib,
		RootPath:       r.options.PackageRootPath,
		SkipValidation: true,
	})
	if err != nil {
		return result.WithError(fmt.Errorf("failed to initialize package installer: %v", err))
	}
	_, err = installer.Install()
	if err != nil {
		return result.WithError(fmt.Errorf("failed to install package: %v", err))
	}
	r.deletePackageHandler = func() error {
		err := installer.Uninstall()

		// by default system package is part of an agent policy and it cannot be uninstalled
		// https://github.com/elastic/elastic-package/blob/5f65dc29811c57454bc7142aaf73725b6d4dc8e6/internal/stack/_static/kibana.yml.tmpl#L62
		if err != nil && pkgManifest.Name != "system" {
			// logging the error as a warning and not returning it since there could be other reasons that could make fail this process
			// for instance being defined a test agent policy where this package is used for debugging purposes
			logger.Warnf("failed to uninstall package %q: %s", pkgManifest.Name, err.Error())
		}
		return nil
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
		return result.WithError(fmt.Errorf("could not create test policy: %w", err))
	}
	r.deleteTestPolicyHandler = func() error {
		logger.Debug("deleting test policy...")
		if err := kib.DeletePolicy(*policy); err != nil {
			return fmt.Errorf("error cleaning up test policy: %w", err)
		}
		return nil
	}

	logger.Debug("adding package data stream to test policy...")
	ds := createPackageDatastream(*policy, *pkgManifest, policyTemplate, *dataStreamManifest, *config)
	if err := kib.AddPackageDataStreamToPolicy(ds); err != nil {
		return result.WithError(fmt.Errorf("could not add data stream config to policy: %w", err))
	}

	// Delete old data
	logger.Debug("deleting old data in data stream...")
	dataStream := fmt.Sprintf(
		"%s-%s-%s",
		ds.Inputs[0].Streams[0].DataStream.Type,
		ds.Inputs[0].Streams[0].DataStream.Dataset,
		ds.Namespace,
	)

	componentTemplatePackage := fmt.Sprintf(
		"%s-%s@package",
		ds.Inputs[0].Streams[0].DataStream.Type,
		ds.Inputs[0].Streams[0].DataStream.Dataset,
	)

	r.wipeDataStreamHandler = func() error {
		logger.Debugf("deleting data in data stream...")
		if err := deleteDataStreamDocs(r.options.API, dataStream); err != nil {
			return fmt.Errorf("error deleting data in data stream: %w", err)
		}
		return nil
	}

	if err := deleteDataStreamDocs(r.options.API, dataStream); err != nil {
		return result.WithError(fmt.Errorf("error deleting old data in data stream: %s: %w", dataStream, err))
	}

	cleared, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return true, errors.New("SIGINT: cancel clearing data")
		}

		hits, err := r.getDocs(dataStream)
		return hits.size() == 0, err
	}, 2*time.Minute)
	if err != nil || !cleared {
		if err == nil {
			err = errors.New("unable to clear previous data")
		}
		return result.WithError(err)
	}

	agents, err := checkEnrolledAgents(kib, ctxt)
	if err != nil {
		return result.WithError(fmt.Errorf("can't check enrolled agents: %w", err))
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
			return fmt.Errorf("error reassigning original policy to agent: %w", err)
		}
		return nil
	}

	policyWithDataStream, err := kib.GetPolicy(policy.ID)
	if err != nil {
		return result.WithError(fmt.Errorf("could not read the policy with data stream: %w", err))
	}

	logger.Debug("assigning package data stream to agent...")
	if err := kib.AssignPolicyToAgent(agent, *policyWithDataStream); err != nil {
		return result.WithError(fmt.Errorf("could not assign policy to agent: %w", err))
	}

	// Signal to the service that the agent is ready (policy is assigned).
	if config.ServiceNotifySignal != "" {
		if err = service.Signal(config.ServiceNotifySignal); err != nil {
			return result.WithError(fmt.Errorf("failed to notify test service: %w", err))
		}
	}

	// Use custom timeout if the service can't collect data immediately.
	waitForDataTimeout := waitForDataDefaultTimeout
	if config.WaitForDataTimeout > 0 {
		waitForDataTimeout = config.WaitForDataTimeout
	}

	// (TODO in future) Optionally exercise service to generate load.
	logger.Debug("checking for expected data in data stream...")
	var hits *hits
	oldHits := 0
	passed, err := waitUntilTrue(func() (bool, error) {
		if signal.SIGINT() {
			return true, errors.New("SIGINT: cancel waiting for policy assigned")
		}

		var err error
		hits, err = r.getDocs(dataStream)

		if config.Assert.HitCount > 0 {
			if hits.size() < config.Assert.HitCount {
				return false, err
			}

			ret := hits.size() == oldHits
			if !ret {
				oldHits = hits.size()
				time.Sleep(4 * time.Second)
			}

			return ret, err
		}
		return hits.size() > 0, err
	}, waitForDataTimeout)

	if err != nil {
		return result.WithError(err)
	}

	if !passed {
		result.FailureMsg = fmt.Sprintf("could not find hits in %s data stream", dataStream)
		return result.WithError(fmt.Errorf("%s", result.FailureMsg))
	}

	syntheticEnabled, err := r.isSyntheticsEnabled(dataStream, componentTemplatePackage)
	if err != nil {
		return result.WithError(fmt.Errorf("failed to check if synthetic source is enabled: %w", err))
	}
	logger.Debugf("data stream %s has synthetics enabled: %t", dataStream, syntheticEnabled)

	docs := hits.getDocs(syntheticEnabled)

	// Validate fields in docs
	// when reroute processors are used, expectedDatasets should be set depends on the processor config
	var expectedDatasets []string
	for _, pipeline := range r.pipelines {
		var esIngestPipeline map[string]any
		err = yaml.Unmarshal(pipeline.Content, &esIngestPipeline)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling ingest pipeline content failed: %w", err)
		}
		processors, _ := esIngestPipeline["processors"].([]any)
		for _, p := range processors {
			processor, ok := p.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unexpected processor %+v", p)
			}
			if reroute, ok := processor["reroute"]; ok {
				if rerouteP, ok := reroute.(ingest.RerouteProcessor); ok {
					expectedDatasets = append(expectedDatasets, rerouteP.Dataset...)
				}
			}
		}
	}

	if expectedDatasets == nil {
		var expectedDataset string
		if ds := r.options.TestFolder.DataStream; ds != "" {
			expectedDataset = getDataStreamDataset(*pkgManifest, *dataStreamManifest)
		} else {
			expectedDataset = pkgManifest.Name + "." + policyTemplateName
		}
		expectedDatasets = []string{expectedDataset}
	}

	fieldsValidator, err := fields.CreateValidatorForDirectory(serviceOptions.DataStreamRootPath,
		fields.WithSpecVersion(pkgManifest.SpecVersion),
		fields.WithNumericKeywordFields(config.NumericKeywordFields),
		fields.WithExpectedDatasets(expectedDatasets),
		fields.WithEnabledImportAllECSSChema(true),
		fields.WithDisableNormalization(syntheticEnabled),
	)
	if err != nil {
		return result.WithError(fmt.Errorf("creating fields validator for data stream failed (path: %s): %w", serviceOptions.DataStreamRootPath, err))
	}
	if err := validateFields(docs, fieldsValidator, dataStream); err != nil {
		return result.WithError(err)
	}

	if syntheticEnabled {
		docs, err = fieldsValidator.SanitizeSyntheticSourceDocs(docs)
		if err != nil {
			return result.WithError(fmt.Errorf("failed to sanitize synthetic source docs: %w", err))
		}
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
			return false, fmt.Errorf("could not list agents: %w", err)
		}

		agents = filterAgents(allAgents, ctxt)
		logger.Debugf("found %d enrolled agent(s)", len(agents))
		if len(agents) == 0 {
			return false, nil // selected agents are unavailable yet
		}
		return true, nil
	}, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("agent enrollment failed: %w", err)
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

	// Add input-level vars
	input := policyTemplate.FindInputByType(streamInput)
	if input != nil {
		r.Inputs[0].Vars = setKibanaVariables(input.Vars, config.Vars)
	}

	// Add package-level vars
	r.Vars = setKibanaVariables(pkg.Vars, config.Vars)

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
		return fmt.Errorf("marshalling sample event failed: %w", err)
	}

	err = os.WriteFile(filepath.Join(path, "sample_event.json"), body, 0644)
	if err != nil {
		return fmt.Errorf("writing sample event failed: %w", err)
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
		return fmt.Errorf("failed to write sample event file: %w", err)
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
