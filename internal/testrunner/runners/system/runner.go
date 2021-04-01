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
	"strings"
	"time"

	es "github.com/elastic/go-elasticsearch/v7"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
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

	// Folder path where log files produced by the service
	// are stored on the Agent container's filesystem.
	serviceLogsAgentDir = "/tmp/service_logs"
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

func (r *runner) TearDown() error {
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
	serviceLogsDir, err := install.ServiceLogsDir()
	if err != nil {
		return result.WithError(errors.Wrap(err, "reading service logs directory failed"))
	}

	files, err := listConfigFiles(r.options.TestFolder.Path)
	if err != nil {
		return result.WithError(errors.Wrap(err, "failed listing test case config files"))
	}
	for _, cfgFile := range files {
		var ctxt servicedeployer.ServiceContext
		ctxt.Name = r.options.TestFolder.Package
		ctxt.Logs.Folder.Local = serviceLogsDir
		ctxt.Logs.Folder.Agent = serviceLogsAgentDir
		ctxt.Test.RunID = createTestRunID()
		testConfig, err := newConfig(filepath.Join(r.options.TestFolder.Path, cfgFile), ctxt)
		if err != nil {
			return result.WithError(errors.Wrapf(err, "unable to load system test case file '%s'", cfgFile))
		}

		var partial []testrunner.TestResult
		if testConfig.Skip == nil {
			partial, err = r.runTest(testConfig, ctxt)
		} else {
			logger.Warnf("skipping %s test for %s/%s: %s (details: %s)",
				TestType, r.options.TestFolder.Package, r.options.TestFolder.DataStream,
				testConfig.Skip.Reason, testConfig.Skip.Link.String())
			result := r.newResult(testConfig.Name())
			partial, err = result.WithSkip(testConfig.Skip)
		}

		results = append(results, partial...)
		if err != nil {
			return results, err
		}
		if err = r.TearDown(); err != nil {
			return results, errors.Wrap(err, "failed to teardown runner")
		}
	}
	return results, nil
}

func createTestRunID() string {
	return fmt.Sprintf("%d", rand.Intn(testRunMaxID-testRunMinID)+testRunMinID)
}

func (r *runner) getDocs(dataStream string) ([]common.MapStr, error) {
	resp, err := r.options.ESClient.Search(
		r.options.ESClient.Search.WithIndex(dataStream),
		r.options.ESClient.Search.WithSort("@timestamp:asc"),
		r.options.ESClient.Search.WithSize(elasticsearchQuerySize),
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
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, errors.Wrap(err, "could not decode search results response")
	}

	numHits := results.Hits.Total.Value
	logger.Debugf("found %d hits in %s data stream", numHits, dataStream)

	var docs []common.MapStr
	for _, hit := range results.Hits.Hits {
		docs = append(docs, hit.Source)
	}

	return docs, nil
}

func (r *runner) runTest(config *testConfig, ctxt servicedeployer.ServiceContext) ([]testrunner.TestResult, error) {
	result := r.newResult(config.Name())

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return result.WithError(errors.Wrap(err, "reading package manifest failed"))
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return result.WithError(errors.Wrap(err, "locating data stream root failed"))
	}
	if !found {
		return result.WithError(errors.New("data stream root not found"))
	}

	dataStreamManifest, err := packages.ReadDataStreamManifest(filepath.Join(dataStreamPath, packages.DataStreamManifestFile))
	if err != nil {
		return result.WithError(errors.Wrap(err, "reading data stream manifest failed"))
	}

	// Setup service.
	logger.Debug("setting up service...")
	serviceDeployer, err := servicedeployer.Factory(servicedeployer.FactoryOptions{
		PackageRootPath:    r.options.PackageRootPath,
		DataStreamRootPath: dataStreamPath,
	})
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
	config, err = newConfig(config.Path, ctxt)
	if err != nil {
		return result.WithError(errors.Wrap(err, "unable to reload system test case configuration"))
	}

	kib, err := kibana.NewClient()
	if err != nil {
		return result.WithError(errors.Wrap(err, "can't create Kibana client"))
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

	ds := createPackageDatastream(*policy, *pkgManifest, *dataStreamManifest, *config)
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
		if err := deleteDataStreamDocs(r.options.ESClient, dataStream); err != nil {
			return errors.Wrap(err, "error deleting data in data stream")
		}
		return nil
	}

	if err := deleteDataStreamDocs(r.options.ESClient, dataStream); err != nil {
		return result.WithError(errors.Wrapf(err, "error deleting old data in data stream: %s", dataStream))
	}

	cleared, err := waitUntilTrue(func() (bool, error) {
		docs, err := r.getDocs(dataStream)
		return len(docs) == 0, err
	}, 2*time.Minute)
	if err != nil || !cleared {
		if err == nil {
			err = errors.New("unable to clear previous data")
		}
		return result.WithError(err)
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

	// (TODO in future) Optionally exercise service to generate load.
	logger.Debug("checking for expected data in data stream...")
	var docs []common.MapStr
	passed, err := waitUntilTrue(func() (bool, error) {
		var err error
		docs, err = r.getDocs(dataStream)
		return len(docs) > 0, err
	}, 10*time.Minute)

	if err != nil {
		return result.WithError(err)
	}

	if !passed {
		result.FailureMsg = fmt.Sprintf("could not find hits in %s data stream", dataStream)
	}

	// Validate fields in docs
	fieldsValidator, err := fields.CreateValidatorForDataStream(dataStreamPath,
		fields.WithNumericKeywordFields(config.NumericKeywordFields))
	if err != nil {
		return result.WithError(errors.Wrapf(err, "creating fields validator for data stream failed (path: %s)", dataStreamPath))
	}

	if err := validateFields(docs, fieldsValidator, dataStream); err != nil {
		return result.WithError(errors.Wrap(err, "failed to validate fields"))
	}

	// Write sample events file from first doc, if requested
	if r.options.GenerateTestResult {
		ds := r.options.TestFolder.DataStream
		dsPath := filepath.Join(r.options.PackageRootPath, "data_stream", ds)
		if err := writeSampleEvent(dsPath, docs[0]); err != nil {
			return result.WithError(errors.Wrap(err, "failed to write sample event file"))
		}
	}

	return result.WithSuccess()
}

func checkEnrolledAgents(client *kibana.Client, ctxt servicedeployer.ServiceContext) ([]kibana.Agent, error) {
	var agents []kibana.Agent
	enrolled, err := waitUntilTrue(func() (bool, error) {
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
	p kibana.Policy,
	pkg packages.PackageManifest,
	ds packages.DataStreamManifest,
	c testConfig,
) kibana.PackageDataStream {
	stream := ds.Streams[getDataStreamIndex(c.Input, ds)]
	streamInput := stream.Input
	r := kibana.PackageDataStream{
		Name:      fmt.Sprintf("%s-%s", pkg.Name, ds.Name),
		Namespace: "ep",
		PolicyID:  p.ID,
		Enabled:   true,
	}

	r.Package.Name = pkg.Name
	r.Package.Title = pkg.Title
	r.Package.Version = pkg.Version

	r.Inputs = []kibana.Input{
		{
			Type:    streamInput,
			Enabled: true,
		},
	}

	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, ds.Name),
			Enabled: true,
			DataStream: kibana.DataStream{
				Type:    ds.Type,
				Dataset: fmt.Sprintf("%s.%s", pkg.Name, ds.Name),
			},
		},
	}

	// Add dataStream-level vars
	dsVars := kibana.Vars{}
	for _, dsVar := range stream.Vars {
		val := dsVar.Default

		cfgVar, exists := c.DataStream.Vars[dsVar.Name]
		if exists {
			// overlay var value from test configuration
			val = cfgVar
		}

		dsVars[dsVar.Name] = kibana.Var{
			Type:  dsVar.Type,
			Value: val,
		}
	}
	streams[0].Vars = dsVars
	r.Inputs[0].Streams = streams

	// Add package-level vars
	pkgVars := kibana.Vars{}
	input := pkg.PolicyTemplates[0].FindInputByType(streamInput)
	if input != nil {
		// copy package-level vars into each input
		input.Vars = append(input.Vars, pkg.Vars...)
		for _, pkgVar := range input.Vars {
			val := pkgVar.Default

			cfgVar, exists := c.Vars[pkgVar.Name]
			if exists {
				// overlay var value from test configuration
				val = cfgVar
			}

			pkgVars[pkgVar.Name] = kibana.Var{
				Type:  pkgVar.Type,
				Value: val,
			}
		}
	}
	r.Inputs[0].Vars = pkgVars

	return r
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

func deleteDataStreamDocs(esClient *es.Client, dataStream string) error {
	body := strings.NewReader(`{ "query": { "match_all": {} } }`)
	_, err := esClient.DeleteByQuery([]string{dataStream}, body)
	if err != nil {
		return err
	}

	return nil
}

func waitUntilTrue(fn func() (bool, error), timeout time.Duration) (bool, error) {
	startTime := time.Now()
	for time.Now().Sub(startTime) < timeout {
		result, err := fn()
		if err != nil {
			return false, err
		}

		if result {
			return true, nil
		}

		time.Sleep(1 * time.Second)
	}

	return false, nil
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
