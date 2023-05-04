// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"errors"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"
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
	kibanaClient    *kibana.Client

	installedPackage *installer.InstalledPackage

	// Execution order of following handlers is defined in runner.tearDown() method.
	removePackageHandler func() error
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

// String returns the name of the test runner.
func (r runner) String() string {
	return "asset loading"
}

// CanRunPerDataStream returns whether this test runner can run on individual
// data streams within the package.
func (r runner) CanRunPerDataStream() bool {
	return false
}

// Setup install the package
func (r *runner) Setup(options testrunner.TestOptions) error {
	r.testFolder = options.TestFolder
	r.packageRootPath = options.PackageRootPath

	logger.Debug("installing package...")
	packageInstaller, err := installer.NewForPackage(installer.Options{
		Kibana:         r.kibanaClient,
		RootPath:       r.packageRootPath,
		SkipValidation: true,
	})
	if err != nil {
		return fmt.Errorf("can't create the package installer: %w", err)
	}
	r.installedPackage, err = packageInstaller.Install()
	if err != nil {
		return fmt.Errorf("can't install the package: %w", err)
	}

	r.removePackageHandler = func() error {
		pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
		if err != nil {
			return fmt.Errorf("reading package manifest failed: %w", err)
		}

		logger.Debug("removing package...")
		err = packageInstaller.Uninstall()

		// by default system package is part of an agent policy and it cannot be uninstalled
		// https://github.com/elastic/elastic-package/blob/5f65dc29811c57454bc7142aaf73725b6d4dc8e6/internal/stack/_static/kibana.yml.tmpl#L62
		if err != nil && pkgManifest.Name != "system" {
			logger.Warnf("failed to uninstall package %q: %s", pkgManifest.Name, err.Error())
		}
		return nil
	}
}

// Run runs the asset loading tests
func (r *runner) Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.testFolder = options.TestFolder
	r.packageRootPath = options.PackageRootPath
	r.kibanaClient = options.KibanaClient

	return r.run()
}

func (r *runner) run() ([]testrunner.TestResult, error) {
	result := testrunner.NewResultComposer(testrunner.TestResult{
		TestType: TestType,
		Package:  r.testFolder.Package,
	})

	if r.kibanaClient == nil {
		return result.WithError(errors.New("missing Kibana client"))
	}

	testConfig, err := newConfig(r.testFolder.Path)
	if err != nil {
		return result.WithError(fmt.Errorf("unable to load asset loading test config file: %w", err))
	}

	if testConfig != nil && testConfig.Skip != nil {
		logger.Warnf("skipping %s test for %s: %s (details: %s)",
			TestType, r.testFolder.Package,
			testConfig.Skip.Reason, testConfig.Skip.Link.String())
		return result.WithSkip(testConfig.Skip)
	}

	expectedAssets, err := packages.LoadPackageAssets(r.packageRootPath)
	if err != nil {
		return result.WithError(fmt.Errorf("could not load expected package assets: %w", err))
	}
	logger.Debugf("Information about installed package: %+v", r.installedPackage)

	results := make([]testrunner.TestResult, 0, len(expectedAssets))
	for _, e := range expectedAssets {
		rc := testrunner.NewResultComposer(testrunner.TestResult{
			Name:       fmt.Sprintf("%s %s is loaded", e.Type, e.ID),
			Package:    r.installedPackage.Name,
			DataStream: e.DataStream,
			TestType:   TestType,
		})

		var testResult []testrunner.TestResult
		if !findActualAsset(r.installedPackage.Assets, e) {
			testResult, _ = rc.WithError(testrunner.ErrTestCaseFailed{
				Reason:  "could not find expected asset",
				Details: fmt.Sprintf("could not find %s asset \"%s\". Assets loaded:\n%s", e.Type, e.ID, formatAssetsAsString(r.installedPackage.Assets)),
			})
		} else {
			testResult, _ = rc.WithSuccess()
		}

		results = append(results, testResult[0])
	}

	return results, nil
}

func (r *runner) TearDown() error {
	if r.removePackageHandler != nil {
		if err := r.removePackageHandler(); err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) TestFolderRequired() bool {
	return false
}

func findActualAsset(actualAssets []packages.Asset, expectedAsset packages.Asset) bool {
	for _, a := range actualAssets {
		if a.Type == expectedAsset.Type && a.ID == expectedAsset.ID {
			return true
		}
	}

	return false
}

func formatAssetsAsString(assets []packages.Asset) string {
	var sb strings.Builder
	for _, asset := range assets {
		sb.WriteString(fmt.Sprintf("- %s\n", asset.String()))
	}
	return sb.String()
}
