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
	logger.Info("Setting up service...")
	serviceRunner, err := servicedeployer.Factory(r.packageRootPath)
	if err != nil {
		return errors.Wrap(err, "could not create service runner")
	}

	ctxt := common.MapStr{}
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
	// TODO: mount ctxt.Get("stdoutFilePath") and ctxt.Get("stderrFilePath")

	// Step 3. Configure package (single data stream) via Ingest Manager APIs.
	im, err := ingestmanager.NewClient(r.stackSettings.kibana.host, r.stackSettings.elasticsearch.username, r.stackSettings.elasticsearch.password)
	if err != nil {
		return errors.Wrap(err, "could not create ingest manager client")
	}

	p := ingestmanager.Policy{
		Name:        fmt.Sprintf("ep-test-system-%s-%s", r.testFolder.Package, r.testFolder.Dataset),
		Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.testFolder.Package, r.testFolder.Dataset),
		Namespace:   "ep",
	}
	policy, err := im.CreatePolicy(p)
	if err != nil {
		return errors.Wrap(err, "could not create policy")
	}
	defer func() {
		time.Sleep(30 * time.Second) // TODO: remove
		if err := im.DeletePolicy(*policy); err != nil {
			logger.Errorf("error cleaning up policy: %s", err)
		}
	}()

	// TODO: build data stream config by taking appropriate vars sections from package manifest + dataset manifest,
	// starting with defaults, then overridding with vars from {dataset}/_dev/test/system/vars.yml. Then treat result
	// as go template and evaulate against ctxt. See expected final structure in im.AddPackageDataStreamToPolicy method.

	if err := im.AddPackageDataStreamToPolicy(*policy, *pkgManifest, *datasetManifest); err != nil {
		return errors.Wrap(err, "could not add dataset config to policy")
	}

	fmt.Println(policy)

	// Step 4. (TODO in future) Optionally exercise service to generate load.

	// Step 5. Assert that there's expected data in data stream.

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
