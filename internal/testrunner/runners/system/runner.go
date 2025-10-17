// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/testrunner"
)

type runner struct {
	profile         *profile.Profile
	repositoryRoot  *os.Root
	packageRootPath string
	kibanaClient    *kibana.Client
	esAPI           *elasticsearch.API
	esClient        *elasticsearch.Client

	dataStreams    []string
	serviceVariant string

	globalTestConfig   testrunner.GlobalRunnerTestConfig
	failOnMissingTests bool
	deferCleanup       time.Duration
	generateTestResult bool
	withCoverage       bool
	coverageType       string

	configFilePath string
	runSetup       bool
	runTearDown    bool
	runTestsOnly   bool

	resourcesManager     *resources.Manager
	serviceStateFilePath string
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

type SystemTestRunnerOptions struct {
	Profile         *profile.Profile
	PackageRootPath string
	RepositoryRoot  *os.Root
	KibanaClient    *kibana.Client
	API             *elasticsearch.API

	// FIXME: Keeping Elasticsearch client to be able to do low-level requests for parameters not supported yet by the API.
	ESClient *elasticsearch.Client

	DataStreams    []string
	ServiceVariant string

	RunSetup       bool
	RunTearDown    bool
	RunTestsOnly   bool
	ConfigFilePath string

	GlobalTestConfig testrunner.GlobalRunnerTestConfig

	FailOnMissingTests bool
	GenerateTestResult bool
	DeferCleanup       time.Duration
	WithCoverage       bool
	CoverageType       string
}

func NewSystemTestRunner(options SystemTestRunnerOptions) *runner {
	r := runner{
		packageRootPath:    options.PackageRootPath,
		kibanaClient:       options.KibanaClient,
		esAPI:              options.API,
		esClient:           options.ESClient,
		profile:            options.Profile,
		dataStreams:        options.DataStreams,
		serviceVariant:     options.ServiceVariant,
		configFilePath:     options.ConfigFilePath,
		runSetup:           options.RunSetup,
		runTestsOnly:       options.RunTestsOnly,
		runTearDown:        options.RunTearDown,
		failOnMissingTests: options.FailOnMissingTests,
		generateTestResult: options.GenerateTestResult,
		deferCleanup:       options.DeferCleanup,
		globalTestConfig:   options.GlobalTestConfig,
		withCoverage:       options.WithCoverage,
		coverageType:       options.CoverageType,
		repositoryRoot:     options.RepositoryRoot,
	}

	r.resourcesManager = resources.NewManager()
	r.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: r.kibanaClient})

	r.serviceStateFilePath = filepath.Join(stateFolderPath(r.profile.ProfilePath), serviceStateFileName)
	return &r
}

// SetupRunner prepares global resources required by the test runner.
func (r *runner) SetupRunner(ctx context.Context) error {
	if r.runTearDown {
		logger.Debug("Skip installing package")
		return nil
	}

	// Install the package before creating the policy, so we control exactly what is being
	// installed.
	logger.Info("Installing package...")
	resourcesOptions := resourcesOptions{
		// Install it unless we are running the tear down only.
		installedPackage: !r.runTearDown,
	}
	_, err := r.resourcesManager.ApplyCtx(ctx, r.resources(resourcesOptions))
	if err != nil {
		return fmt.Errorf("can't install the package: %w", err)
	}

	return nil
}

// TearDownRunner cleans up any global test runner resources. It must be called
// after the test runner has finished executing all its tests.
func (r *runner) TearDownRunner(ctx context.Context) error {
	logger.Info("Uninstalling package...")
	resourcesOptions := resourcesOptions{
		// Keep it installed only if we were running setup, or tests only.
		installedPackage: r.runSetup || r.runTestsOnly,
	}
	_, err := r.resourcesManager.ApplyCtx(ctx, r.resources(resourcesOptions))
	if err != nil {
		return err
	}
	return nil
}

func (r *runner) GetTests(ctx context.Context) ([]testrunner.Tester, error) {
	var folders []testrunner.TestFolder
	manifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed (path: %s): %w", r.packageRootPath, err)
	}

	hasDataStreams, err := testrunner.PackageHasDataStreams(manifest)
	if err != nil {
		return nil, fmt.Errorf("cannot determine if package has data streams: %w", err)
	}

	if r.runSetup || r.runTearDown || r.runTestsOnly {
		_, err := os.Stat(r.serviceStateFilePath)
		logger.Debugf("Service state data exists in %s: %v", r.serviceStateFilePath, !os.IsNotExist(err))
		if r.runSetup && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to run --setup, required to tear down previous setup")
		}
		if r.runTestsOnly && os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to run tests with --no-provision, setup first with --setup")
		}
		if r.runTearDown && os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to run --tear-down, setup not found")
		}
	} else {
		if _, err = os.Stat(r.serviceStateFilePath); !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to run tests, required to tear down previous state run (path: %s)", r.serviceStateFilePath)
		}
	}

	var serviceState ServiceState
	if r.runTearDown || r.runTestsOnly {
		serviceState, err = readServiceStateData(r.serviceStateFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read service state: %w", err)
		}
	}

	if hasDataStreams {
		var dataStreams []string
		if r.runSetup || r.runTearDown || r.runTestsOnly {
			configFilePath := r.configFilePath
			if r.runTearDown || r.runTestsOnly {
				configFilePath = serviceState.ConfigFilePath
			}
			dataStream := testrunner.ExtractDataStreamFromPath(configFilePath, r.packageRootPath)
			dataStreams = append(dataStreams, dataStream)
		} else if len(r.dataStreams) > 0 {
			dataStreams = r.dataStreams
		}

		folders, err = testrunner.FindTestFolders(r.packageRootPath, dataStreams, r.Type())
		if err != nil {
			return nil, fmt.Errorf("unable to determine test folder paths: %w", err)
		}

		if r.failOnMissingTests && len(folders) == 0 {
			if len(dataStreams) > 0 {
				return nil, fmt.Errorf("no %s tests found for %s data stream(s)", r.Type(), strings.Join(dataStreams, ","))
			}
			return nil, fmt.Errorf("no %s tests found", r.Type())
		}
	} else {
		folders, err = testrunner.FindTestFolders(r.packageRootPath, nil, r.Type())
		if err != nil {
			return nil, fmt.Errorf("unable to determine test folder paths: %w", err)
		}
		if r.failOnMissingTests && len(folders) == 0 {
			return nil, fmt.Errorf("no %s tests found", r.Type())
		}
	}

	if r.runSetup || r.runTearDown || r.runTestsOnly {
		// variant flag is not checked here since there are packages that do not have variants
		if len(folders) != 1 {
			return nil, fmt.Errorf("wrong number of test folders (expected 1): %d", len(folders))
		}
	}

	var testers []testrunner.Tester
	for _, t := range folders {
		var variants []string
		var cfgFiles []string

		if r.runTestsOnly || r.runTearDown {
			variants = []string{serviceState.VariantName}
			cfgFiles = []string{filepath.Base(serviceState.ConfigFilePath)}
		} else {
			variants, err = r.getAllVariants(t)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve variants from %s: %w", t.Path, err)
			}

			cfgFiles, err = r.getAllConfigFiles(t)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve config files from %s: %w", t.Path, err)
			}
		}

		for _, variant := range variants {
			for _, config := range cfgFiles {
				logger.Debugf("System runner: data stream %q config file %q variant %q", t.DataStream, config, variant)
				tester, err := NewSystemTester(SystemTesterOptions{
					Profile:            r.profile,
					PackageRootPath:    r.packageRootPath,
					KibanaClient:       r.kibanaClient,
					API:                r.esAPI,
					ESClient:           r.esClient,
					TestFolder:         t,
					ServiceVariant:     variant,
					GenerateTestResult: r.generateTestResult,
					DeferCleanup:       r.deferCleanup,
					RunSetup:           r.runSetup,
					RunTestsOnly:       r.runTestsOnly,
					RunTearDown:        r.runTearDown,
					ConfigFileName:     config,
					GlobalTestConfig:   r.globalTestConfig,
					WithCoverage:       r.withCoverage,
					CoverageType:       r.coverageType,
				})
				if err != nil {
					return nil, fmt.Errorf(
						"failed to create system runner for sdata stream %q variant %q config file %q: %w",
						t.DataStream, variant, config, err)
				}
				testers = append(testers, tester)
			}
		}
	}
	return testers, nil
}

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

func (r *runner) resources(opts resourcesOptions) resources.Resources {
	return resources.Resources{
		&resources.FleetPackage{
			PackageRootPath: r.packageRootPath,
			Absent:          !opts.installedPackage,
			Force:           opts.installedPackage, // Force re-installation, in case there are code changes in the same package version.
			RepositoryRoot:  r.repositoryRoot,
		},
	}
}

func (r *runner) selectVariants(variantsFile *servicedeployer.VariantsFile) []string {
	if variantsFile == nil || variantsFile.Variants == nil {
		return []string{""} // empty variants file switches to no-variant mode
	}

	var variantNames []string
	for k := range variantsFile.Variants {
		if r.serviceVariant != "" && r.serviceVariant != k {
			continue
		}
		variantNames = append(variantNames, k)
	}
	return variantNames
}

func (r *runner) getAllVariants(folder testrunner.TestFolder) ([]string, error) {
	var variants []string
	dataStreamPath, found, err := packages.FindDataStreamRootForPath(folder.Path)
	if err != nil {
		return nil, fmt.Errorf("locating data stream root failed: %w", err)
	}
	if found {
		logger.Debugf("Running system tests for data stream %q", folder.DataStream)
	} else {
		logger.Debug("Running system tests for package")
	}
	devDeployPath, err := servicedeployer.FindDevDeployPath(servicedeployer.FactoryOptions{
		PackageRootPath:    r.packageRootPath,
		DataStreamRootPath: dataStreamPath,
		DevDeployDir:       DevDeployDir,
	})
	switch {
	case errors.Is(err, os.ErrNotExist):
		variants = r.selectVariants(nil)
	case err != nil:
		return nil, fmt.Errorf("failed fo find service deploy path: %w", err)
	default:
		variantsFile, err := servicedeployer.ReadVariantsFile(devDeployPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("can't read service variant: %w", err)
		}
		variants = r.selectVariants(variantsFile)
	}
	if r.serviceVariant != "" && len(variants) == 0 {
		return nil, fmt.Errorf("not found variant definition %q", r.serviceVariant)
	}

	if r.runSetup {
		// variant information in runTestOnly or runTearDown modes is retrieved from serviceOptions (file in setup dir)
		if len(variants) > 1 {
			return nil, fmt.Errorf("a variant must be selected or trigger the test in no-variant mode (available variants: %s)", strings.Join(variants, ", "))
		}
		if len(variants) == 1 && variants[0] == "" {
			logger.Debug("No variant mode")
		}
	}

	return variants, nil
}

func (r *runner) getAllConfigFiles(folder testrunner.TestFolder) ([]string, error) {
	var cfgFiles []string
	var err error
	if r.configFilePath != "" {
		allCfgFiles, err := listConfigFiles(filepath.Dir(r.configFilePath))
		if err != nil {
			return nil, fmt.Errorf("failed listing test case config cfgFiles: %w", err)
		}
		baseFile := filepath.Base(r.configFilePath)
		for _, cfg := range allCfgFiles {
			if cfg == baseFile {
				cfgFiles = append(cfgFiles, baseFile)
			}
		}
	} else {
		cfgFiles, err = listConfigFiles(folder.Path)
		if err != nil {
			return nil, fmt.Errorf("failed listing test case config cfgFiles: %w", err)
		}
	}
	return cfgFiles, nil
}
