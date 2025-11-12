// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/testrunner"
)

type tester struct {
	testFolder       testrunner.TestFolder
	packageRootPath  string
	kibanaClient     *kibana.Client
	resourcesManager *resources.Manager
	globalTestConfig testrunner.GlobalRunnerTestConfig
	withCoverage     bool
	coverageType     string
	repositoryRoot   *os.Root
}

type AssetTesterOptions struct {
	TestFolder       testrunner.TestFolder
	PackageRootPath  string
	KibanaClient     *kibana.Client
	GlobalTestConfig testrunner.GlobalRunnerTestConfig
	WithCoverage     bool
	CoverageType     string
	RepositoryRoot   *os.Root
}

func NewAssetTester(options AssetTesterOptions) *tester {
	tester := tester{
		testFolder:       options.TestFolder,
		packageRootPath:  options.PackageRootPath,
		kibanaClient:     options.KibanaClient,
		globalTestConfig: options.GlobalTestConfig,
		withCoverage:     options.WithCoverage,
		coverageType:     options.CoverageType,
		repositoryRoot:   options.RepositoryRoot,
	}

	manager := resources.NewManager()
	manager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: options.KibanaClient})
	tester.resourcesManager = manager

	return &tester
}

// Ensures that runner implements testrunner.Tester interface
var _ testrunner.Tester = new(tester)

// Type returns the type of test that can be run by this test runner.
func (r *tester) Type() testrunner.TestType {
	return TestType
}

// String returns the name of the test runner.
func (r tester) String() string {
	return "asset loading"
}

// Parallel indicates if this tester can run in parallel or not.
func (r tester) Parallel() bool {
	// Not supported yet parallel tests even if it is indicated in the global config r.globalTestConfig
	return false
}

// Run runs the asset loading tests
func (r *tester) Run(ctx context.Context) ([]testrunner.TestResult, error) {
	return r.run(ctx)
}

func (r *tester) resources(installedPackage bool) resources.Resources {
	return resources.Resources{
		&resources.FleetPackage{
			PackageRootPath: r.packageRootPath,
			Absent:          !installedPackage,
			Force:           installedPackage, // Force re-installation, in case there are code changes in the same package version.
			RepositoryRoot:  r.repositoryRoot,
		},
	}
}

func (r *tester) run(ctx context.Context) ([]testrunner.TestResult, error) {
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

	skipConfigs := []*testrunner.SkipConfig{r.globalTestConfig.Skip}
	if testConfig != nil {
		skipConfigs = append(skipConfigs, testConfig.Skip)
	}

	if skip := testrunner.AnySkipConfig(skipConfigs...); skip != nil {
		logger.Warnf("skipping %s test for %s/%s: %s (details: %s)",
			TestType, r.testFolder.Package, r.testFolder.DataStream,
			skip.Reason, skip.Link)
		return result.WithSkip(skip)
	}

	logger.Debug("installing package...")
	_, err = r.resourcesManager.ApplyCtx(ctx, r.resources(true))
	if err != nil {
		return result.WithError(fmt.Errorf("can't install the package: %w", err))
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return result.WithError(fmt.Errorf("cannot read the package manifest from %s: %w", r.packageRootPath, err))
	}
	installedPackage, err := r.kibanaClient.GetPackage(ctx, manifest.Name)
	if err != nil {
		return result.WithError(fmt.Errorf("cannot get installed package %q: %w", manifest.Name, err))
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
			Package:    r.testFolder.Package,
			DataStream: e.DataStream,
			TestType:   TestType,
		})

		var tr []testrunner.TestResult
		if !findActualAsset(installedAssets, e) {
			tr, _ = rc.WithError(testrunner.ErrTestCaseFailed{
				Reason:  "could not find expected asset",
				Details: fmt.Sprintf("could not find %s asset \"%s\". Assets loaded:\n%s", e.Type, e.ID, formatAssetsAsString(installedAssets)),
			})
		} else {
			tr, _ = rc.WithSuccess()
		}
		result := tr[0]
		if r.withCoverage && e.SourcePath != "" {
			result.Coverage, err = testrunner.GenerateBaseFileCoverageReport(rc.CoveragePackageName(), e.SourcePath, r.coverageType, true)
			if err != nil {
				tr, _ = rc.WithError(testrunner.ErrTestCaseFailed{
					Reason:  "could not generate test coverage",
					Details: fmt.Sprintf("could not generate test coverage for asset in %s: %v", e.SourcePath, err),
				})
				result = tr[0]
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func (r *tester) TearDown(ctx context.Context) error {
	// Avoid cancellations during cleanup.
	cleanupCtx := context.WithoutCancel(ctx)

	logger.Debug("removing package...")
	_, err := r.resourcesManager.ApplyCtx(cleanupCtx, r.resources(false))
	if err != nil {
		return err
	}

	return nil
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
