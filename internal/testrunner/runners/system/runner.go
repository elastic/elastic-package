// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/testrunner"
)

type runner struct {
	profile         *profile.Profile
	packageRootPath string
	kibanaClient    *kibana.Client
	esAPI           *elasticsearch.API

	dataStreams    []string
	serviceVariant string

	failOnMissingTests         bool
	generateTestResult         bool
	runIndependentElasticAgent bool
	deferCleanup               time.Duration

	configFilePath string
	runSetup       bool
	runTearDown    bool
	runTestsOnly   bool

	resourcesManager *resources.Manager
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

type SystemTestRunnerOptions struct {
	Profile         *profile.Profile
	PackageRootPath string
	KibanaClient    *kibana.Client
	API             *elasticsearch.API

	DataStreams    []string
	ServiceVariant string

	RunSetup       bool
	RunTearDown    bool
	RunTestsOnly   bool
	ConfigFilePath string

	FailOnMissingTests         bool
	GenerateTestResult         bool
	RunIndependentElasticAgent bool
	DeferCleanup               time.Duration
}

func NewSystemTestRunner(options SystemTestRunnerOptions) *runner {
	r := runner{
		packageRootPath:            options.PackageRootPath,
		kibanaClient:               options.KibanaClient,
		esAPI:                      options.API,
		profile:                    options.Profile,
		dataStreams:                options.DataStreams,
		serviceVariant:             options.ServiceVariant,
		configFilePath:             options.ConfigFilePath,
		runSetup:                   options.RunSetup,
		runTestsOnly:               options.RunTestsOnly,
		runTearDown:                options.RunTearDown,
		failOnMissingTests:         options.FailOnMissingTests,
		generateTestResult:         options.GenerateTestResult,
		runIndependentElasticAgent: options.RunIndependentElasticAgent,
		deferCleanup:               options.DeferCleanup,
	}

	r.resourcesManager = resources.NewManager()
	r.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: r.kibanaClient})
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
	logger.Debug("Installing package...")
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
	logger.Debug("Uninstalling package...")
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

	configFilePath := r.configFilePath

	if hasDataStreams {
		var dataStreams []string
		if r.runSetup || r.runTearDown || r.runTestsOnly {
			if r.runTearDown || r.runTestsOnly {
				configFilePath, err = readConfigFileFromState(r.profile.ProfilePath)
				if err != nil {
					return nil, fmt.Errorf("failed to get config file from state: %w", err)
				}
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

	// TODO: Return a Tester per each combination of variant plus configuration file in each data stream / folder
	var testers []testrunner.Tester
	for _, t := range folders {
		testers = append(testers, NewSystemTester(SystemTesterOptions{
			Profile:                    r.profile,
			PackageRootPath:            r.packageRootPath,
			KibanaClient:               r.kibanaClient,
			API:                        r.esAPI,
			TestFolder:                 t,
			ServiceVariant:             r.serviceVariant,
			GenerateTestResult:         r.generateTestResult,
			DeferCleanup:               r.deferCleanup,
			RunSetup:                   r.runSetup,
			RunTestsOnly:               r.runTestsOnly,
			RunTearDown:                r.runTearDown,
			ConfigFilePath:             r.configFilePath,
			RunIndependentElasticAgent: r.runIndependentElasticAgent,
		}))
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
			RootPath: r.packageRootPath,
			Absent:   !opts.installedPackage,
			Force:    opts.installedPackage, // Force re-installation, in case there are code changes in the same package version.
		},
	}
}
