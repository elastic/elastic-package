// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	es "github.com/elastic/go-elasticsearch/v7"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana/ingestmanager"
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
)

type runner struct {
	options       testrunner.TestOptions
	stackSettings stackSettings

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
	r.stackSettings = getStackSettingsFromEnv()

	return r.run()
}

func (r *runner) TearDown() error {
	if r.resetAgentPolicyHandler != nil {
		if err := r.resetAgentPolicyHandler(); err != nil {
			return err
		}
	}

	if r.deleteTestPolicyHandler != nil {
		if err := r.deleteTestPolicyHandler(); err != nil {
			return err
		}
	}

	if r.shutdownServiceHandler != nil {
		if err := r.shutdownServiceHandler(); err != nil {
			return err
		}
	}

	if r.wipeDataStreamHandler != nil {
		if err := r.wipeDataStreamHandler(); err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) run() ([]testrunner.TestResult, error) {
	result := testrunner.TestResult{
		TestType:   TestType,
		Package:    r.options.TestFolder.Package,
		DataStream: r.options.TestFolder.DataStream,
	}
	startTime := time.Now()
	resultsWith := func(tr testrunner.TestResult, err error) ([]testrunner.TestResult, error) {
		tr.TimeElapsed = time.Now().Sub(startTime)
		if err == nil {
			return []testrunner.TestResult{tr}, nil
		}

		if tcf, ok := err.(testrunner.ErrTestCaseFailed); ok {
			tr.FailureMsg = tcf.Reason
			tr.FailureDetails = tcf.Details
			return []testrunner.TestResult{tr}, nil
		}

		tr.ErrorMsg = err.Error()
		return []testrunner.TestResult{tr}, err
	}

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "reading package manifest failed"))
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "locating data stream root failed"))
	}
	if !found {
		return resultsWith(result, errors.New("data stream root not found"))
	}

	dataStreamManifest, err := packages.ReadDataStreamManifest(filepath.Join(dataStreamPath, packages.DataStreamManifestFile))
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "reading data stream manifest failed"))
	}

	serviceLogsDir, err := install.ServiceLogsDir()
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "reading service logs directory failed"))
	}

	// Step 1. Setup service.
	// Step 1a. (Deferred) Tear down service.
	logger.Debug("setting up service...")
	serviceDeployer, err := servicedeployer.Factory(r.options.PackageRootPath)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not create service runner"))
	}

	var ctxt servicedeployer.ServiceContext
	ctxt.Name = r.options.TestFolder.Package
	ctxt.Logs.Folder.Local = serviceLogsDir

	service, err := serviceDeployer.SetUp(ctxt)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not setup service"))
	}
	ctxt = service.Context()

	r.shutdownServiceHandler = func() error {
		logger.Debug("tearing down service...")
		if err := service.TearDown(); err != nil {
			return errors.Wrap(err, "error tearing down service")
		}

		return nil
	}

	// Step 2. Configure package (single data stream) via Ingest Manager APIs.
	im, err := ingestmanager.NewClient(r.stackSettings.kibana.host, r.stackSettings.elasticsearch.username, r.stackSettings.elasticsearch.password)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not create ingest manager client"))
	}

	logger.Debug("creating test policy...")
	testTime := time.Now().Format("20060102T15:04:05Z")
	p := ingestmanager.Policy{
		Name:        fmt.Sprintf("ep-test-system-%s-%s-%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream, testTime),
		Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream),
		Namespace:   "ep",
	}
	policy, err := im.CreatePolicy(p)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not create test policy"))
	}
	r.deleteTestPolicyHandler = func() error {
		logger.Debug("deleting test policy...")
		if err := im.DeletePolicy(*policy); err != nil {
			return errors.Wrap(err, "error cleaning up test policy")
		}
		return nil
	}

	testConfig, err := newConfig(r.options.TestFolder.Path, ctxt)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "unable to load system test configuration"))
	}

	logger.Debug("adding package data stream to test policy...")
	ds := createPackageDatastream(*policy, *pkgManifest, *dataStreamManifest, *testConfig)
	if err := im.AddPackageDataStreamToPolicy(ds); err != nil {
		return resultsWith(result, errors.Wrap(err, "could not add data stream config to policy"))
	}

	// Get enrolled agent ID
	agents, err := im.ListAgents()
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not list agents"))
	}
	if agents == nil || len(agents) == 0 {
		return resultsWith(result, errors.New("no agents found"))
	}
	agent := agents[0]
	origPolicy := ingestmanager.Policy{
		ID: agent.PolicyID,
	}

	// Delete old data
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

	logger.Debug("deleting old data in data stream...")
	if err := deleteDataStreamDocs(r.options.ESClient, dataStream); err != nil {
		return resultsWith(result, errors.Wrapf(err, "error deleting old data in data stream: %s", dataStream))
	}

	// Assign policy to agent
	logger.Debug("assigning package data stream to agent...")
	if err := im.AssignPolicyToAgent(agent, *policy); err != nil {
		return resultsWith(result, errors.Wrap(err, "could not assign policy to agent"))
	}
	r.resetAgentPolicyHandler = func() error {
		logger.Debug("reassigning original policy back to agent...")
		if err := im.AssignPolicyToAgent(agent, origPolicy); err != nil {
			return errors.Wrap(err, "error reassigning original policy to agent")
		}
		return nil
	}

	fieldsValidator, err := fields.CreateValidatorForDataStream(dataStreamPath,
		fields.WithNumericKeywordFields(testConfig.NumericKeywordFields))
	if err != nil {
		return resultsWith(result, errors.Wrapf(err, "creating fields validator for data stream failed (path: %s)", dataStreamPath))
	}

	// Step 4. (TODO in future) Optionally exercise service to generate load.
	logger.Debug("checking for expected data in data stream...")
	passed, err := waitUntilTrue(func() (bool, error) {
		resp, err := r.options.ESClient.Search(
			r.options.ESClient.Search.WithIndex(dataStream),
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
		if numHits == 0 {
			return false, nil
		}

		var multiErr multierror.Error
		for _, hit := range results.Hits.Hits {
			if message, err := hit.Source.GetValue("error.message"); err != common.ErrKeyNotFound {
				multiErr = append(multiErr, errors.New(message.(string)))
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
	}, 2*time.Minute)
	if err != nil {
		return resultsWith(result, err)
	}

	if !passed {
		result.FailureMsg = fmt.Sprintf("could not find hits in %s data stream", dataStream)
	}
	return resultsWith(result, nil)
}

func getStackSettingsFromEnv() stackSettings {
	s := stackSettings{}

	s.elasticsearch.host = os.Getenv("ELASTIC_PACKAGE_ELASTICSEARCH_HOST")
	if s.elasticsearch.host == "" {
		s.elasticsearch.host = "http://localhost:9200"
	}

	s.elasticsearch.username = os.Getenv("ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME")
	s.elasticsearch.password = os.Getenv("ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD")

	s.kibana.host = os.Getenv("ELASTIC_PACKAGE_KIBANA_HOST")
	if s.kibana.host == "" {
		s.kibana.host = "http://localhost:5601"
	}

	return s
}

func createPackageDatastream(
	p ingestmanager.Policy,
	pkg packages.PackageManifest,
	ds packages.DataStreamManifest,
	c testConfig,
) ingestmanager.PackageDataStream {
	stream := ds.Streams[getDataStreamIndex(c.Input, ds)]
	streamInput := stream.Input
	r := ingestmanager.PackageDataStream{
		Name:      fmt.Sprintf("%s-%s", pkg.Name, ds.Name),
		Namespace: "ep",
		PolicyID:  p.ID,
		Enabled:   true,
	}

	r.Package.Name = pkg.Name
	r.Package.Title = pkg.Title
	r.Package.Version = pkg.Version

	r.Inputs = []ingestmanager.Input{
		{
			Type:    streamInput,
			Enabled: true,
		},
	}

	streams := []ingestmanager.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, ds.Name),
			Enabled: true,
			DataStream: ingestmanager.DataStream{
				Type:    ds.Type,
				Dataset: fmt.Sprintf("%s.%s", pkg.Name, ds.Name),
			},
		},
	}

	// Add dataStream-level vars
	dsVars := ingestmanager.Vars{}
	for _, dsVar := range stream.Vars {
		val := dsVar.Default

		cfgVar, exists := c.DataStream.Vars[dsVar.Name]
		if exists {
			// overlay var value from test configuration
			val = cfgVar
		}

		dsVars[dsVar.Name] = ingestmanager.Var{
			Type:  dsVar.Type,
			Value: val,
		}
	}
	streams[0].Vars = dsVars
	r.Inputs[0].Streams = streams

	// Add package-level vars
	pkgVars := ingestmanager.Vars{}
	input := pkg.PolicyTemplates[0].FindInputByType(streamInput)
	if input != nil {
		for _, pkgVar := range input.Vars {
			val := pkgVar.Default

			cfgVar, exists := c.Vars[pkgVar.Name]
			if exists {
				// overlay var value from test configuration
				val = cfgVar
			}

			pkgVars[pkgVar.Name] = ingestmanager.Var{
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
