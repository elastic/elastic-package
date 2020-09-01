// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
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
		options.TestFolder,
		options.PackageRootPath,
		getStackSettingsFromEnv(),
	}
	return r.run()
}

func (r *runner) run() error {
	testConfig, err := newConfig(r.testFolder.Path)
	if err != nil {
		return errors.Wrap(err, "unable to load system test configuration")
	}

	pkgManifest, err := packages.ReadPackageManifest(filepath.Join(r.packageRootPath, packages.PackageManifestFile))
	if err != nil {
		return errors.Wrap(err, "reading package manifest failed")
	}

	datasetPath, found, err := packages.FindDatasetRootForPath(r.testFolder.Path)
	if err != nil {
		return errors.Wrap(err, "locating dataset root failed")
	}
	if !found {
		return errors.New("dataset root not found")
	}

	datasetManifest, err := packages.ReadDatasetManifest(filepath.Join(datasetPath, packages.DatasetManifestFile))
	if err != nil {
		return errors.Wrap(err, "reading dataset manifest failed")
	}

	// Step 1. Setup service.
	// Step 1a. (Deferred) Tear down service.
	logger.Info("setting up service...")
	serviceRunner, err := servicedeployer.Factory(r.packageRootPath)
	if err != nil {
		return errors.Wrap(err, "could not create service runner")
	}

	tempDir, err := install.TempDir()
	if err != nil {
		return errors.Wrap(err, "could not get temporary folder")
	}

	ctxt := common.MapStr{}
	ctxt.Put("service.name", r.testFolder.Package)
	ctxt.Put("tempdir", tempDir)

	ctxt, err = serviceRunner.SetUp(ctxt)
	defer func() {
		logger.Info("tearing down service...")
		if err := serviceRunner.TearDown(ctxt); err != nil {
			logger.Errorf("error tearing down service: %s", err)
		}
	}()
	if err != nil {
		return errors.Wrap(err, "could not setup service")
	}

	// Step 2. Setup agent and enroll it with Fleet
	// TODO: Should this be part of the service factory, since the agent might need
	// to be deployed "close to" the service? So for docker-compose based services,
	// we could use the Agent Docker image, for TF-based services, we use TF to
	// deploy the agent "near" the service, etc.?
	// TODO: mount tempdir

	// Step 3. Configure package (single data stream) via Ingest Manager APIs.
	im, err := ingestmanager.NewClient(r.stackSettings.kibana.host, r.stackSettings.elasticsearch.username, r.stackSettings.elasticsearch.password)
	if err != nil {
		return errors.Wrap(err, "could not create ingest manager client")
	}

	logger.Info("creating test policy...")
	p := ingestmanager.Policy{
		Name:        fmt.Sprintf("ep-test-system-%s-%s", r.testFolder.Package, r.testFolder.Dataset),
		Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.testFolder.Package, r.testFolder.Dataset),
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

	// TODO: build data stream Config by taking appropriate vars sections from package manifest + dataset manifest,
	// starting with defaults, then overridding with vars from {dataset}/_dev/test/system/vars.yml. Then treat result
	// as go template and evaulate against ctxt. See expected final structure in im.AddPackageDataStreamToPolicy method.

	logger.Info("adding package datastream to test policy...")
	if err := im.AddPackageDataStreamToPolicy(createPackageDatastream(*policy, *pkgManifest, *datasetManifest, *testConfig)); err != nil {
		return errors.Wrap(err, "could not add dataset Config to policy")
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

	// Assign policy to agent
	logger.Info("assigning package datastream to agent...")
	if err := im.AssignPolicyToAgent(agent, *policy); err != nil {
		return errors.Wrap(err, "could not assign policy to agent")
	}
	defer func() {
		logger.Debug("reassigning original policy back to agent...")
		if err := im.AssignPolicyToAgent(agent, origPolicy); err != nil {
			logger.Errorf("error reassigning original policy to agent: %s", err)
		}
	}()

	// Step 4. (TODO in future) Optionally exercise service to generate load.

	// Step 5. Assert that there's expected data in data stream.

	defer func() {
		sleepFor := 1 * time.Minute
		logger.Debugf("waiting for %s before destructing...", sleepFor)
		time.Sleep(sleepFor)
	}()
	return nil
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

func createPackageDatastream(p ingestmanager.Policy, pkg packages.PackageManifest, ds packages.DatasetManifest, c config) ingestmanager.PackageDatastream {
	streamInput := ds.Streams[0].Input
	r := ingestmanager.PackageDatastream{
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
			DataStream: ingestmanager.Datastream{
				Type:    ds.Type,
				Dataset: fmt.Sprintf("%s.%s", pkg.Name, ds.Name),
			},
		},
	}

	// Add dataset-level vars
	dsVars := ingestmanager.Vars{}
	for _, dsVar := range ds.Streams[0].Vars {
		val := dsVar.Default

		cfgVar, exists := c.Dataset.Vars[dsVar.Name]
		if exists {
			// overlay var value from test configuration
			val = cfgVar
		}

		dsVars[dsVar.Name] = ingestmanager.VarType{
			Type:  dsVar.Type,
			Value: val,
		}
	}
	streams[0].Vars = dsVars
	r.Inputs[0].Streams = streams

	// Add package-level vars
	pkgVars := ingestmanager.Vars{}
	input := pkg.ConfigTemplates[0].FindInputByType(streamInput)
	if input != nil {
		for _, pkgVar := range input.Vars {
			val := pkgVar.Default

			cfgVar, exists := c.Vars[pkgVar.Name]
			if exists {
				// overlay var value from test configuration
				val = cfgVar
			}

			pkgVars[pkgVar.Name] = ingestmanager.VarType{
				Type:  pkgVar.Type,
				Value: val,
			}
		}
	}
	r.Inputs[0].Vars = pkgVars

	return r
}
