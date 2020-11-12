// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"fmt"
	"path/filepath"
	"time"

	es "github.com/elastic/go-elasticsearch/v7"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/kibana/ingestmanager"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterRunner(&runner{})
}

const (
	// TestType defining asset loading tests
	TestType testrunner.TestType = "asset"
)

type runner struct {
	testFolder      testrunner.TestFolder
	packageRootPath string
	stackSettings   testrunner.StackSettings
	esClient        *es.Client

	// Execution order of following handlers is defined in runner.tearDown() method.
	deleteTestPolicyHandler func() error
}

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

// String returns the name of the test runner.
func (r runner) String() string {
	return "asset loading"
}

// Run runs the asset loading tests
func (r runner) Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.testFolder = options.TestFolder
	r.packageRootPath = options.PackageRootPath
	r.stackSettings = testrunner.GetStackSettingsFromEnv()
	r.esClient = options.ESClient

	return r.run()
}

func (r *runner) run() ([]testrunner.TestResult, error) {
	result := testrunner.TestResult{
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
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

	pkgManifest, err := packages.ReadPackageManifest(filepath.Join(r.packageRootPath, packages.PackageManifestFile))
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "reading package manifest failed"))
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.testFolder.Path)
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

	// Step 1. Configure package (single data stream) via Ingest Manager APIs.
	im, err := ingestmanager.NewClient(r.stackSettings.Kibana.Host, r.stackSettings.Elasticsearch.Username, r.stackSettings.Elasticsearch.Password)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not create ingest manager client"))
	}

	logger.Debug("creating test policy...")
	testTime := time.Now().Format("20060102T15:04:05Z")
	p := ingestmanager.Policy{
		Name:        fmt.Sprintf("ep-test-system-%s-%s-%s", r.testFolder.Package, r.testFolder.DataStream, testTime),
		Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.testFolder.Package, r.testFolder.DataStream),
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

	logger.Debug("adding package data stream to test policy...")
	ds := createPackageDatastream(*policy, *pkgManifest, *dataStreamManifest)
	if err := im.AddPackageDataStreamToPolicy(ds); err != nil {
		return resultsWith(result, errors.Wrap(err, "could not add data stream config to policy"))
	}
	// TODO: defer remove integration

	// TODO: Verify that data stream assets are loaded as expected
	// index templates
	// kibana saved objects

	return resultsWith(result, nil)
}

func (r *runner) TearDown() error {
	if r.deleteTestPolicyHandler != nil {
		if err := r.deleteTestPolicyHandler(); err != nil {
			return err
		}
	}

	return nil
}

func createPackageDatastream(
	p ingestmanager.Policy,
	pkg packages.PackageManifest,
	ds packages.DataStreamManifest,
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
				Type:    ds.Type,
				Dataset: fmt.Sprintf("%s.%s", pkg.Name, ds.Name),
			},
		},
	}

	// Add dataStream-level vars
	dsVars := ingestmanager.Vars{}
	for _, dsVar := range ds.Streams[0].Vars {
		val := dsVar.Default
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
			pkgVars[pkgVar.Name] = ingestmanager.Var{
				Type:  pkgVar.Type,
				Value: val,
			}
		}
	}
	r.Inputs[0].Vars = pkgVars

	return r
}
