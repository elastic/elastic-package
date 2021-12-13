// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

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

	// Execution order of following handlers is defined in runner.tearDown() method.
	removePackageHandler func() error
}

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

// Run runs the asset loading tests
func (r runner) Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.testFolder = options.TestFolder
	r.packageRootPath = options.PackageRootPath

	return r.run()
}

func (r *runner) run() ([]testrunner.TestResult, error) {
	result := testrunner.NewResultComposer(testrunner.TestResult{
		TestType: TestType,
		Package:  r.testFolder.Package,
	})

	testConfig, err := newConfig(r.testFolder.Path)
	if err != nil {
		return result.WithError(errors.Wrap(err, "unable to load asset loading test config file"))

	}

	if testConfig != nil && testConfig.Skip != nil {
		logger.Warnf("skipping %s test for %s: %s (details: %s)",
			TestType, r.testFolder.Package,
			testConfig.Skip.Reason, testConfig.Skip.Link.String())
		return result.WithSkip(testConfig.Skip)
	}

	logger.Debug("installing package...")
	manifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return result.WithError(errors.Wrapf(err, "reading package manifest failed (path: %s)", r.packageRootPath))
	}

	packageInstaller, err := installer.CreateForManifest(*manifest)
	if err != nil {
		return result.WithError(errors.Wrap(err, "can't create the package installer"))
	}
	installedPackage, err := packageInstaller.Install()
	if err != nil {
		return result.WithError(errors.Wrap(err, "can't install the package"))
	}

	r.removePackageHandler = func() error {
		logger.Debug("removing package...")
		if err := packageInstaller.Uninstall(); err != nil {
			return errors.Wrap(err, "error cleaning up package")
		}
		return nil
	}

	expectedAssets, err := packages.LoadPackageAssets(r.packageRootPath)
	if err != nil {
		return result.WithError(errors.Wrap(err, "could not load expected package assets"))
	}

	results := make([]testrunner.TestResult, 0, len(expectedAssets))
	for _, e := range expectedAssets {
		rc := testrunner.NewResultComposer(testrunner.TestResult{
			Name:       fmt.Sprintf("%s %s is loaded", e.Type, e.ID),
			Package:    installedPackage.Manifest.Name,
			DataStream: e.DataStream,
			TestType:   TestType,
		})

		var r []testrunner.TestResult
		if !findActualAsset(installedPackage.Assets, e) {
			r, _ = rc.WithError(testrunner.ErrTestCaseFailed{
				Reason:  "could not find expected asset",
				Details: fmt.Sprintf("could not find %s asset \"%s\". Assets loaded:\n%s", e.Type, e.ID, formatAssetsAsString(installedPackage.Assets)),
			})
		} else {
			r, _ = rc.WithSuccess()
		}

		results = append(results, r[0])
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
