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

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana/ingestmanager"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system/servicedeployer"
)

func init() {
	testrunner.RegisterRunner(TestType, Run)
}

const (
	// TestType defining system tests
	TestType testrunner.TestType = "system"
)

type runner struct {
	testFolder      testrunner.TestFolder
	packageRootPath string
	stackSettings   stackSettings
	esClient        *es.Client

	// Execution order of following handlers is defined in runner.tearDown() method.
	resetAgentPolicyHandler func()
	shutdownServiceHandler  func()
	wipeDataStreamHandler   func()
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

// Run runs the system tests defined under the given folder
func Run(options testrunner.TestOptions) error {
	r := runner{
		testFolder:      options.TestFolder,
		packageRootPath: options.PackageRootPath,
		stackSettings:   getStackSettingsFromEnv(),
		esClient:        options.ESClient,
	}
	defer r.tearDown()
	return r.run()
}

func (r *runner) run() error {
	pkgManifest, err := packages.ReadPackageManifest(filepath.Join(r.packageRootPath, packages.PackageManifestFile))
	if err != nil {
		return errors.Wrap(err, "reading package manifest failed")
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.testFolder.Path)
	if err != nil {
		return errors.Wrap(err, "locating data stream root failed")
	}
	if !found {
		return errors.New("dataStream root not found")
	}

	dataStreamManifest, err := packages.ReadDataStreamManifest(filepath.Join(dataStreamPath, packages.DataStreamManifestFile))
	if err != nil {
		return errors.Wrap(err, "reading data stream manifest failed")
	}

	serviceLogsDir, err := install.ServiceLogsDir()
	if err != nil {
		return errors.Wrap(err, "reading service logs directory failed")
	}

	// Step 1. Setup service.
	// Step 1a. (Deferred) Tear down service.
	logger.Info("setting up service...")
	serviceDeployer, err := servicedeployer.Factory(r.packageRootPath)
	if err != nil {
		return errors.Wrap(err, "could not create service runner")
	}

	var ctxt servicedeployer.ServiceContext
	ctxt.Name = r.testFolder.Package
	ctxt.Logs.Folder.Local = serviceLogsDir

	service, err := serviceDeployer.SetUp(ctxt)
	if err != nil {
		return errors.Wrap(err, "could not setup service")
	}
	ctxt = service.Context()

	r.shutdownServiceHandler = func() {
		logger.Info("tearing down service...")
		if err := service.TearDown(); err != nil {
			logger.Errorf("error tearing down service: %s", err)
		}
	}

	// Step 2. Configure package (single data stream) via Ingest Manager APIs.
	im, err := ingestmanager.NewClient(r.stackSettings.kibana.host, r.stackSettings.elasticsearch.username, r.stackSettings.elasticsearch.password)
	if err != nil {
		return errors.Wrap(err, "could not create ingest manager client")
	}

	logger.Info("creating test policy...")
	testTime := time.Now().Format("20060102T15:04:05Z")
	p := ingestmanager.Policy{
		Name:        fmt.Sprintf("ep-test-system-%s-%s-%s", r.testFolder.Package, r.testFolder.DataStream, testTime),
		Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.testFolder.Package, r.testFolder.DataStream),
		Namespace:   "ep",
	}
	policy, err := im.CreatePolicy(p)
	if err != nil {
		return errors.Wrap(err, "could not create test policy")
	}
	defer func() {
		logger.Debug("deleting test policy...")
		if err := im.DeletePolicy(*policy); err != nil {
			logger.Errorf("error cleaning up test policy: %s", err)
		}
	}()

	testConfig, err := newConfig(r.testFolder.Path, ctxt)
	if err != nil {
		return errors.Wrap(err, "unable to load system test configuration")
	}

	logger.Info("adding package data stream to test policy...")
	ds := createPackageDatastream(*policy, *pkgManifest, *dataStreamManifest, *testConfig)
	if err := im.AddPackageDataStreamToPolicy(ds); err != nil {
		return errors.Wrap(err, "could not add data stream config to policy")
	}

	// Get enrolled agent ID
	agents, err := im.ListAgents()
	if err != nil {
		return errors.Wrap(err, "could not list agents")
	}
	if agents == nil || len(agents) == 0 {
		return errors.New("no agents found")
	}
	agent := agents[0]
	origPolicy := ingestmanager.Policy{
		ID: agent.PolicyID,
	}

	// Delete old data
	dataStream := fmt.Sprintf(
		"%s-%s-%s",
		ds.Inputs[0].Streams[0].DataStream.Type,
		ds.Inputs[0].Streams[0].DataStream.DataStream,
		ds.Namespace,
	)

	r.wipeDataStreamHandler = func() {
		logger.Debugf("deleting data in data stream...")
		if err := deleteDataStreamDocs(r.esClient, dataStream); err != nil {
			logger.Errorf("error deleting data in data stream", err)
		}
	}

	logger.Info("deleting old data in data stream...")
	if err := deleteDataStreamDocs(r.esClient, dataStream); err != nil {
		return errors.Wrapf(err, "error deleting old data in data stream: %s", dataStream)
	}

	// Assign policy to agent
	logger.Info("assigning package data streamto agent...")
	if err := im.AssignPolicyToAgent(agent, *policy); err != nil {
		return errors.Wrap(err, "could not assign policy to agent")
	}
	r.resetAgentPolicyHandler = func() {
		logger.Debug("reassigning original policy back to agent...")
		if err := im.AssignPolicyToAgent(agent, origPolicy); err != nil {
			logger.Errorf("error reassigning original policy to agent: %s", err)
		}
	}

	// Step 4. (TODO in future) Optionally exercise service to generate load.
	logger.Info("checking for expected data in data stream...")
	passed, err := waitUntilTrue(func() (bool, error) {
		resp, err := r.esClient.Search(
			r.esClient.Search.WithIndex(dataStream),
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
			}
		}

		if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
			return false, errors.Wrap(err, "could not decode search results response")
		}

		hits := results.Hits.Total.Value
		logger.Debugf("found %d hits in %s data stream", hits, dataStream)
		return hits > 0, nil
	}, 2*time.Minute)

	if err != nil {
		return errors.Wrap(err, "could not check for expected data in data stream")
	}

	if passed {
		fmt.Printf("System test for %s/%s data stream passed!\n", r.testFolder.Package, r.testFolder.DataStream)
	} else {
		fmt.Printf("System test for %s/%s data stream failed\n", r.testFolder.Package, r.testFolder.DataStream)
		return fmt.Errorf("system test for %s/%s data stream failed", r.testFolder.Package, r.testFolder.DataStream)
	}
	return nil
}

func (r *runner) tearDown() {
	if r.resetAgentPolicyHandler != nil {
		r.resetAgentPolicyHandler()
	}

	if r.shutdownServiceHandler != nil {
		r.shutdownServiceHandler()
	}

	if r.wipeDataStreamHandler != nil {
		r.wipeDataStreamHandler()
	}
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
	streamInput := ds.Streams[0].Input
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
				Type:       ds.Type,
				DataStream: fmt.Sprintf("%s.%s", pkg.Name, ds.Name),
			},
		},
	}

	// Add dataStream-level vars
	dsVars := ingestmanager.Vars{}
	for _, dsVar := range ds.Streams[0].Vars {
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
