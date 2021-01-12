// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"encoding/json"
	"fmt"
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

type stackSettings struct {
	elasticsearch struct {
		host     string
		username string
		password string
	}
	kibana struct {
		host string
	}
}

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

// String returns the human-friendly name of the test runner.
func (r *runner) String() string {
	return "system"
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

type resultComposer struct {
	testrunner.TestResult
	startTime time.Time
}

func (r *runner) newResult(name string) resultComposer {
	return resultComposer{
		TestResult: testrunner.TestResult{
			TestType:   TestType,
			Name:       name,
			Package:    r.options.TestFolder.Package,
			DataStream: r.options.TestFolder.DataStream,
		},
		startTime: time.Now(),
	}
}

func (rc *resultComposer) withError(err error) ([]testrunner.TestResult, error) {
	rc.TimeElapsed = time.Now().Sub(rc.startTime)
	if err == nil {
		return []testrunner.TestResult{rc.TestResult}, nil
	}

	if tcf, ok := err.(testrunner.ErrTestCaseFailed); ok {
		rc.FailureMsg += tcf.Reason
		rc.FailureDetails += tcf.Details
		return []testrunner.TestResult{rc.TestResult}, nil
	}

	rc.ErrorMsg += err.Error()
	return []testrunner.TestResult{rc.TestResult}, err
}

func (rc *resultComposer) withSuccess() ([]testrunner.TestResult, error) {
	return rc.withError(nil)
}

func (r *runner) run() (results []testrunner.TestResult, err error) {
	result := r.newResult("(init)")
	serviceLogsDir, err := install.ServiceLogsDir()
	if err != nil {
		return result.withError(errors.Wrap(err, "reading service logs directory failed"))
	}

	files, err := listConfigFiles(r.options.TestFolder.Path)
	if err != nil {
		return result.withError(errors.Wrap(err, "failed listing test case config files"))
	}
	for _, cfgFile := range files {
		var ctxt servicedeployer.ServiceContext
		ctxt.Name = r.options.TestFolder.Package
		ctxt.Logs.Folder.Local = serviceLogsDir
		ctxt.Logs.Folder.Agent = serviceLogsAgentDir
		testConfig, err := newConfig(filepath.Join(r.options.TestFolder.Path, cfgFile), ctxt)
		if err != nil {
			return result.withError(errors.Wrapf(err, "unable to load system test case file '%s'", cfgFile))
		}
		partial, err := r.runTest(testConfig, ctxt)
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

func (r *runner) hasNumDocs(
	dataStream string,
	fieldsValidator *fields.Validator,
	checker func(int) bool) func() (bool, error) {
	return func() (bool, error) {
		resp, err := r.options.ESClient.Search(
			r.options.ESClient.Search.WithIndex(dataStream),
			r.options.ESClient.Search.WithSort("@timestamp:asc"),
			r.options.ESClient.Search.WithSize(elasticsearchQuerySize),
		)
		if err != nil {
			return false, errors.Wrap(err, "could not search data stream")
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
			return false, errors.Wrap(err, "could not decode search results response")
		}

		numHits := results.Hits.Total.Value
		logger.Debugf("found %d hits in %s data stream", numHits, dataStream)
		if !checker(numHits) {
			return false, nil
		}

		var multiErr multierror.Error
		for _, hit := range results.Hits.Hits {
			if message, err := hit.Source.GetValue("error.message"); err != common.ErrKeyNotFound {
				multiErr = append(multiErr, fmt.Errorf("found error.message in event: %v", message))
				continue
			}

			errs := fieldsValidator.ValidateDocumentMap(hit.Source)
			if errs != nil {
				multiErr = append(multiErr, errs...)
				continue
			}
		}

		if len(multiErr) > 0 {
			multiErr = multiErr.Unique()
			return false, testrunner.ErrTestCaseFailed{
				Reason:  fmt.Sprintf("one or more errors found in documents stored in %s data stream", dataStream),
				Details: multiErr.Error(),
			}
		}
		return true, nil
	}
}

func (r *runner) runTest(config *testConfig, ctxt servicedeployer.ServiceContext) ([]testrunner.TestResult, error) {
	result := r.newResult(config.Name())

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return result.withError(errors.Wrap(err, "reading package manifest failed"))
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return result.withError(errors.Wrap(err, "locating data stream root failed"))
	}
	if !found {
		return result.withError(errors.New("data stream root not found"))
	}

	dataStreamManifest, err := packages.ReadDataStreamManifest(filepath.Join(dataStreamPath, packages.DataStreamManifestFile))
	if err != nil {
		return result.withError(errors.Wrap(err, "reading data stream manifest failed"))
	}

	// Setup service.
	logger.Debug("setting up service...")
	serviceDeployer, err := servicedeployer.Factory(r.options.PackageRootPath)
	if err != nil {
		return result.withError(errors.Wrap(err, "could not create service runner"))
	}

	if config.Service != "" {
		ctxt.Name = config.Service
	}
	service, err := serviceDeployer.SetUp(ctxt)
	if err != nil {
		return result.withError(errors.Wrap(err, "could not setup service"))
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
		return result.withError(errors.Wrap(err, "unable to reload system test case configuration"))
	}

	// Configure package (single data stream) via Ingest Manager APIs.
	kib, err := kibana.NewClient()
	if err != nil {
		return result.withError(errors.Wrap(err, "could not create ingest manager client"))
	}

	logger.Debug("creating test policy...")
	testTime := time.Now().Format("20060102T15:04:05Z")
	p := kibana.Policy{
		Name:        fmt.Sprintf("ep-test-system-%s-%s-%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream, testTime),
		Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream),
		Namespace:   "ep",
	}
	policy, err := kib.CreatePolicy(p)
	if err != nil {
		return result.withError(errors.Wrap(err, "could not create test policy"))
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
		return result.withError(errors.Wrap(err, "could not add data stream config to policy"))
	}

	// Get enrolled agent ID
	agents, err := kib.ListAgents()
	if err != nil {
		return result.withError(errors.Wrap(err, "could not list agents"))
	}
	if agents == nil || len(agents) == 0 {
		return result.withError(errors.New("no agents found"))
	}
	agent := agents[0]
	origPolicy := kibana.Policy{
		ID: agent.PolicyID,
	}

	// Create field validator
	fieldsValidator, err := fields.CreateValidatorForDataStream(dataStreamPath,
		fields.WithNumericKeywordFields(config.NumericKeywordFields))
	if err != nil {
		return result.withError(errors.Wrapf(err, "creating fields validator for data stream failed (path: %s)", dataStreamPath))
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
		return result.withError(errors.Wrapf(err, "error deleting old data in data stream: %s", dataStream))
	}

	cleared, err := waitUntilTrue(r.hasNumDocs(dataStream, fieldsValidator, func(n int) bool {
		return n == 0
	}), 2*time.Minute)
	if err != nil || !cleared {
		if err == nil {
			err = errors.New("unable to clear previous data")
		}
		return result.withError(err)
	}

	// Assign policy to agent
	logger.Debug("assigning package data stream to agent...")
	if err := kib.AssignPolicyToAgent(agent, *policy); err != nil {
		return result.withError(errors.Wrap(err, "could not assign policy to agent"))
	}
	r.resetAgentPolicyHandler = func() error {
		logger.Debug("reassigning original policy back to agent...")
		if err := kib.AssignPolicyToAgent(agent, origPolicy); err != nil {
			return errors.Wrap(err, "error reassigning original policy to agent")
		}
		return nil
	}

	// (TODO in future) Optionally exercise service to generate load.
	logger.Debug("checking for expected data in data stream...")
	passed, err := waitUntilTrue(r.hasNumDocs(dataStream, fieldsValidator, func(n int) bool {
		return n > 0
	}), 2*time.Minute)

	if err != nil {
		return result.withError(err)
	}

	if !passed {
		result.FailureMsg = fmt.Sprintf("could not find hits in %s data stream", dataStream)
	}
	return result.withSuccess()
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
