// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/resources"
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
	testFolder       testrunner.TestFolder
	packageRootPath  string
	kibanaClient     *kibana.Client
	resourcesManager *resources.Manager
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

	manager := resources.NewManager()
	manager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: r.kibanaClient})
	r.resourcesManager = manager

	return r.run(ctx)
}

func (r *runner) resources(packageInstalled bool) resources.Resources {
	return resources.Resources{
		&resources.FleetPackage{
			RootPath: r.packageRootPath,
			Absent:   !packageInstalled,
			Force:    true,
		},
	}
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
	applyResult, err := r.resourcesManager.ApplyCtx(ctx, r.resources(true))
	// FIXME: Apply doesn't wrap context errors.
	if errors.Is(err, context.Canceled) {
		// Installation interrupted, at this point the package may have been installed, try to remove it for cleanup.
		_, err := r.resourcesManager.ApplyCtx(context.WithoutCancel(ctx), r.resources(false))
		if err != nil {
			logger.Debugf("error while removing package after installation interrupted: %s", err)
		}
	}
	if err != nil {
		for _, result := range applyResult {
			if result.Err() != nil {
				logger.Debugf(result.Err().Error())
			}
		}
		return result.WithError(fmt.Errorf("can't install the package: %w", err))
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return result.WithError(fmt.Errorf("cannot read the package manifest from %s: %w", r.packageRootPath, err))
	}
	installedPackage, err := r.kibanaClient.GetPackage(ctx, manifest.Name)
	if err != nil {
		return result.WithError(fmt.Errorf("cannot get installed package %q", manifest.Name, err))
	}
	installedAssets := installedPackage.Assets()

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
		if !findActualAsset(installedAssets, e) {
			r, _ = rc.WithError(testrunner.ErrTestCaseFailed{
				Reason:  "could not find expected asset",
				Details: fmt.Sprintf("could not find %s asset \"%s\". Assets loaded:\n%s", e.Type, e.ID, formatAssetsAsString(installedAssets)),
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

	_, err := r.resourcesManager.ApplyCtx(cleanupCtx, r.resources(false))
	if err != nil {
		return err
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
