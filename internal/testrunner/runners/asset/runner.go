// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"

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

	// Execution order of following handlers is defined in runner.tearDown() method.
	removePackageHandler func(context.Context) error
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

// CanRunSetupTeardownIndependent returns whether this test runner can run setup or
// teardown process independent.
func (r *runner) CanRunSetupTeardownIndependent() bool {
	return false
}

// Run runs the asset loading tests
func (r *runner) Run(ctx context.Context, options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.testFolder = options.TestFolder
	r.packageRootPath = options.PackageRootPath
	r.kibanaClient = options.KibanaClient

	return r.run(ctx)
}

func (r *runner) run(ctx context.Context) ([]testrunner.TestResult, error) {
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

	logger.Debug("installing package...")
	packageInstaller, err := installer.NewForPackage(installer.Options{
		Kibana:         r.kibanaClient,
		RootPath:       r.packageRootPath,
		SkipValidation: true,
	})
	if err != nil {
		return result.WithError(fmt.Errorf("can't create the package installer: %w", err))
	}

	removePackageHandler := func(ctx context.Context) error {
		pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
		if err != nil {
			return fmt.Errorf("reading package manifest failed: %w", err)
		}

		kibanaVersion, err := r.kibanaClient.Version()
		if err != nil {
			return fmt.Errorf("failed to retrieve kibana version: %w", err)
		}
		stackVersion, err := semver.NewVersion(kibanaVersion.Version())
		if err != nil {
			return fmt.Errorf("failed to parse kibana version: %w", err)
		}

		if stackVersion.LessThan(semver.MustParse("8.0.0")) && pkgManifest.Name == "system" {
			// in Elastic stack 7.* , system package is installed in the default Agent policy and it cannot be deleted
			// error: system is installed by default and cannot be removed
			logger.Debugf("skip uninstalling %s package", pkgManifest.Name)
			return nil
		}

		logger.Debug("removing package...")
		err = packageInstaller.Uninstall(ctx)
		if err != nil {
			// logging the error as a warning and not returning it since there could be other reasons that could make fail this process
			// for instance being defined a test agent policy where this package is used for debugging purposes
			logger.Warnf("failed to uninstall package %q: %s", pkgManifest.Name, err.Error())
		}
		return nil
	}

	installedPackage, err := packageInstaller.Install(ctx)
	if errors.Is(err, context.Canceled) {
		// Installation interrupted, at this point the package may have been installed, try to remove it for cleanup.
		err := removePackageHandler(context.WithoutCancel(ctx))
		if err != nil {
			logger.Debugf("error while removing package after installation interrupted: %s", err)
		}
	}
	if err != nil {
		return result.WithError(fmt.Errorf("can't install the package: %w", err))
	}

	r.removePackageHandler = removePackageHandler

	// No Elasticsearch asset is created when an Input package is installed through the API.
	// This would require to create a Agent policy and add that input package to the Agent policy.
	// As those input packages could have some required fields, it would also require to add
	// configuration files as in system tests to fill those fields.
	// In these tests, mainly it is required to test Kibana assets, therefore it is not added
	// support for Elasticsearch assets in input packages.
	// Related issue: https://github.com/elastic/elastic-package/issues/1623
	expectedAssets, err := packages.LoadPackageAssets(r.packageRootPath)
	if err != nil {
		return result.WithError(fmt.Errorf("could not load expected package assets: %w", err))
	}

	results := make([]testrunner.TestResult, 0, len(expectedAssets))
	for _, e := range expectedAssets {
		rc := testrunner.NewResultComposer(testrunner.TestResult{
			Name:       fmt.Sprintf("%s %s is loaded", e.Type, e.ID),
			Package:    installedPackage.Name,
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

func (r *runner) TearDown(ctx context.Context) error {
	// Avoid cancellations during cleanup.
	cleanupCtx := context.WithoutCancel(ctx)

	if r.removePackageHandler != nil {
		if err := r.removePackageHandler(cleanupCtx); err != nil {
			return err
		}
		r.removePackageHandler = nil
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
