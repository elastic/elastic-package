// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"context"
	"fmt"
	"strings"

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

	dataStreams        []string
	failOnMissingTests bool

	configFilePath string
	runSetup       bool
	runTearDown    bool
	runTestsOnly   bool

	resourcesManager *resources.Manager
}

type SystemTestRunnerOptions struct {
	Profile         *profile.Profile
	PackageRootPath string
	KibanaClient    *kibana.Client

	RunSetup       bool
	RunTearDown    bool
	RunTestsOnly   bool
	ConfigFilePath string

	DataStreams        []string
	FailOnMissingTests bool
}

func NewSystemTestRunner(options SystemTestRunnerOptions) *runner {
	r := runner{
		packageRootPath:    options.PackageRootPath,
		kibanaClient:       options.KibanaClient,
		runSetup:           options.RunSetup,
		runTestsOnly:       options.RunTestsOnly,
		runTearDown:        options.RunTearDown,
		profile:            options.Profile,
		dataStreams:        options.DataStreams,
		failOnMissingTests: options.FailOnMissingTests,
		configFilePath:     options.ConfigFilePath,
	}

	r.resourcesManager = resources.NewManager()
	r.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: r.kibanaClient})
	// TODO: check if logic in initRun could be moved to this constructor
	return &r
}

// SetupRunner prepares global resources required by the test runner.
func (r *runner) SetupRunner(ctx context.Context) error {
	if r.runTearDown {
		logger.Debug("Skip installing package")
	} else {
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

func (r *runner) GetTests(ctx context.Context) ([]testrunner.TestFolder, error) {
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
				configFilePath, err = testrunner.ReadConfigFileFromState(r.profile.ProfilePath)
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

	return folders, nil
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

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
