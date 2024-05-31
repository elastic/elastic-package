// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/agentdeployer"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/wait"
)

const (
	testRunMaxID = 99999
	testRunMinID = 10000

	checkFieldsBody = `{
		"fields": ["*"],
		"runtime_mappings": {
		  "my_ignored": {
			"type": "keyword",
			"script": {
			  "source": "for (def v : params['_fields']._ignored.values) { emit(v); }"
			}
		  }
		},
		"aggs": {
		  "all_ignored": {
			"filter": {
			  "exists": {
				"field": "_ignored"
			  }
			},
			"aggs": {
			  "ignored_fields": {
				"terms": {
				  "size": 100,
				  "field": "my_ignored"
				}
			  },
			  "ignored_docs": {
				"top_hits": {
				  "size": 5
				}
			  }
			}
		  }
		}
	  }`
	DevDeployDir = "_dev/deploy"
)

func init() {
	testrunner.RegisterRunner(&runner{})
}

const (
	// TestType defining system tests
	TestType testrunner.TestType = "system"

	// Maximum number of events to query.
	elasticsearchQuerySize = 500

	// ServiceLogsAgentDir is folder path where log files produced by the service
	// are stored on the Agent container's filesystem.
	ServiceLogsAgentDir = "/tmp/service_logs"

	waitForDataDefaultTimeout = 10 * time.Minute
)

type logsRegexp struct {
	includes *regexp.Regexp
	excludes []*regexp.Regexp
}

type logsByContainer struct {
	containerName string
	patterns      []logsRegexp
}

var (
	errorPatterns = []logsByContainer{
		{
			containerName: "elastic-agent",
			patterns: []logsRegexp{
				{
					includes: regexp.MustCompile("^Cannot index event publisher.Event"),
					excludes: []*regexp.Regexp{
						// this regex is excluded to ensure that logs coming from the `system` package installed by default are not taken into account
						regexp.MustCompile(`action \[indices:data\/write\/bulk\[s\]\] is unauthorized for API key id \[.*\] of user \[.*\] on indices \[.*\], this action is granted by the index privileges \[.*\]`),
					},
				},
				{
					includes: regexp.MustCompile("->(FAILED|DEGRADED)"),

					// this regex is excluded to avoid a regresion in 8.11 that can make a component to pass to a degraded state during some seconds after reassigning or removing a policy
					excludes: []*regexp.Regexp{
						regexp.MustCompile(`Component state changed .* \(HEALTHY->DEGRADED\): Degraded: pid .* missed .* check-in`),
					},
				},
			},
		},
	}
	enableIndependentAgents = environment.WithElasticPackagePrefix("TEST_ENABLE_INDEPENDENT_AGENT")
)

type runner struct {
	options   testrunner.TestOptions
	pipelines []ingest.Pipeline

	dataStreamPath   string
	cfgFiles         []string
	variants         []string
	stackVersion     kibana.VersionInfo
	locationManager  *locations.LocationManager
	resourcesManager *resources.Manager

	serviceStateFilePath string

	// Execution order of following handlers is defined in runner.TearDown() method.
	removeAgentHandler        func(context.Context) error
	deleteTestPolicyHandler   func(context.Context) error
	cleanTestScenarioHandler  func(context.Context) error
	resetAgentPolicyHandler   func(context.Context) error
	resetAgentLogLevelHandler func(context.Context) error
	shutdownServiceHandler    func(context.Context) error
	shutdownAgentHandler      func(context.Context) error
	wipeDataStreamHandler     func(context.Context) error
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

// String returns the human-friendly name of the test runner.
func (r *runner) String() string {
	return "system"
}

// SetupRunner prepares global resources required by the test runner.
func (r *runner) SetupRunner(ctx context.Context, options testrunner.TestOptions) error {
	r.options = options

	r.resourcesManager = resources.NewManager()
	r.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: r.options.KibanaClient})

	if r.options.RunTearDown {
		logger.Debug("Skip installing package")
	} else {
		// Install the package before creating the policy, so we control exactly what is being
		// installed.
		logger.Debug("Installing package...")
		resourcesOptions := resourcesOptions{
			// Install it unless we are running the tear down only.
			installedPackage: !r.options.RunTearDown,
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
	logger.Debugf("Uninstalling package...")
	resourcesOptions := resourcesOptions{
		// Keep it installed only if we were running setup, or tests only.
		installedPackage: r.options.RunSetup || r.options.RunTestsOnly,
	}
	_, err := r.resourcesManager.ApplyCtx(ctx, r.resources(resourcesOptions))
	if err != nil {
		return err
	}
	return nil
}

// CanRunPerDataStream returns whether this test runner can run on individual
// data streams within the package.
func (r *runner) CanRunPerDataStream() bool {
	return true
}

func (r *runner) TestFolderRequired() bool {
	return true
}

// CanRunSetupTeardownIndependent returns whether this test runner can run setup or
// teardown process independent.
func (r *runner) CanRunSetupTeardownIndependent() bool {
	return true
}

// Run runs the system tests defined under the given folder
func (r *runner) Run(ctx context.Context, options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.options.TestFolder = options.TestFolder
	if !r.options.RunSetup && !r.options.RunTearDown && !r.options.RunTestsOnly {
		return r.run(ctx)
	}

	result := r.newResult("(init)")
	if err := r.initRun(); err != nil {
		return result.WithError(err)
	}

	if r.options.RunSetup {
		// variant information in runTestOnly or runTearDown modes is retrieved from serviceOptions (file in setup dir)
		if len(r.variants) > 1 {
			return result.WithError(fmt.Errorf("a variant must be selected or trigger the test in no-variant mode (available variants: %s)", strings.Join(r.variants, ", ")))
		}
		if len(r.variants) == 0 {
			logger.Debug("No variant mode")
		}
	}

	_, err := os.Stat(r.serviceStateFilePath)
	logger.Debugf("Service state data exists in %s: %v", r.serviceStateFilePath, !os.IsNotExist(err))
	if r.options.RunSetup && !os.IsNotExist(err) {
		return result.WithError(fmt.Errorf("failed to run --setup, required to tear down previous setup"))
	}
	if r.options.RunTestsOnly && os.IsNotExist(err) {
		return result.WithError(fmt.Errorf("failed to run tests with --no-provision, setup first with --setup"))
	}
	if r.options.RunTearDown && os.IsNotExist(err) {
		return result.WithError(fmt.Errorf("failed to run --tear-down, setup not found"))
	}

	var serviceStateData ServiceState
	if !r.options.RunSetup {
		serviceStateData, err = r.readServiceStateData()
		if err != nil {
			return result.WithError(fmt.Errorf("failed to read service state: %w", err))
		}
	}

	configFile := r.options.ConfigFilePath
	variant := r.variants[0]
	if r.options.RunTestsOnly || r.options.RunTearDown {
		configFile = serviceStateData.ConfigFilePath
		variant = serviceStateData.VariantName

		logger.Infof("Using test config file from setup dir: %q", configFile)
		logger.Infof("Using variant from service setup dir: %q", variant)
	}

	svcInfo, err := r.createServiceInfo()
	if err != nil {
		return result.WithError(err)
	}

	testConfig, err := newConfig(configFile, svcInfo, variant)
	if err != nil {
		return nil, fmt.Errorf("unable to load system test case file '%s': %w", configFile, err)
	}
	logger.Debugf("Using config: %q", testConfig.Name())

	resultName := ""
	switch {
	case r.options.RunSetup:
		resultName = "setup"
	case r.options.RunTearDown:
		resultName = "teardown"
	case r.options.RunTestsOnly:
		resultName = "tests"
	}
	result = r.newResult(fmt.Sprintf("%s - %s", resultName, testConfig.Name()))

	scenario, err := r.prepareScenario(ctx, testConfig, svcInfo)
	if r.options.RunSetup && err != nil {
		tdErr := r.tearDownTest(ctx)
		if tdErr != nil {
			logger.Errorf("failed to tear down runner: %s", tdErr.Error())
		}

		setupDirErr := r.removeServiceStateFile()
		if setupDirErr != nil {
			logger.Error(err.Error())
		}
		return result.WithError(err)
	}

	if r.options.RunTestsOnly {
		if err != nil {
			return result.WithError(fmt.Errorf("failed to prepare scenario: %w", err))
		}
		results, err := r.validateTestScenario(ctx, result, scenario, testConfig)
		tdErr := r.tearDownTest(ctx)
		if tdErr != nil {
			logger.Errorf("failed to tear down runner: %s", tdErr.Error())
		}
		return results, err

	}

	if r.options.RunTearDown {
		if err != nil {
			logger.Errorf("failed to prepare scenario: %s", err.Error())
			logger.Errorf("continue with the tear down process")
		}
		if err := r.tearDownTest(ctx); err != nil {
			return result.WithError(err)
		}

		err := r.removeServiceStateFile()
		if err != nil {
			return result.WithError(err)
		}
	}

	return result.WithSuccess()
}

type resourcesOptions struct {
	installedPackage bool
}

func (r *runner) resources(opts resourcesOptions) resources.Resources {
	return resources.Resources{
		&resources.FleetPackage{
			RootPath: r.options.PackageRootPath,
			Absent:   !opts.installedPackage,
			Force:    opts.installedPackage, // Force re-installation, in case there are code changes in the same package version.
		},
	}
}

func (r *runner) createAgentOptions(policyName string) agentdeployer.FactoryOptions {
	return agentdeployer.FactoryOptions{
		Profile:            r.options.Profile,
		PackageRootPath:    r.options.PackageRootPath,
		DataStreamRootPath: r.dataStreamPath,
		DevDeployDir:       DevDeployDir,
		Type:               agentdeployer.TypeTest,
		StackVersion:       r.stackVersion.Version(),
		PackageName:        r.options.TestFolder.Package,
		DataStream:         r.options.TestFolder.DataStream,
		PolicyName:         policyName,
		RunTearDown:        r.options.RunTearDown,
		RunTestsOnly:       r.options.RunTestsOnly,
		RunSetup:           r.options.RunSetup,
	}
}

func (r *runner) createServiceOptions(variantName string) servicedeployer.FactoryOptions {
	return servicedeployer.FactoryOptions{
		Profile:                r.options.Profile,
		PackageRootPath:        r.options.PackageRootPath,
		DataStreamRootPath:     r.dataStreamPath,
		DevDeployDir:           DevDeployDir,
		Variant:                variantName,
		Type:                   servicedeployer.TypeTest,
		StackVersion:           r.stackVersion.Version(),
		PackageName:            r.options.TestFolder.Package,
		DataStream:             r.options.TestFolder.DataStream,
		RunTearDown:            r.options.RunTearDown,
		RunTestsOnly:           r.options.RunTestsOnly,
		RunSetup:               r.options.RunSetup,
		DeployIndependentAgent: r.options.RunIndependentElasticAgent,
	}
}

func (r *runner) createAgentInfo(policy *kibana.Policy, config *testConfig, runID string, agentManifest packages.Agent) (agentdeployer.AgentInfo, error) {
	var info agentdeployer.AgentInfo

	info.Name = r.options.TestFolder.Package
	info.Logs.Folder.Agent = ServiceLogsAgentDir
	info.Test.RunID = runID

	folderName := fmt.Sprintf("agent-%s", r.options.TestFolder.Package)
	if r.options.TestFolder.DataStream != "" {
		folderName = fmt.Sprintf("%s-%s", folderName, r.options.TestFolder.DataStream)
	}
	folderName = fmt.Sprintf("%s-%s", folderName, runID)

	dirPath, err := agentdeployer.CreateServiceLogsDir(r.options.Profile, folderName)
	if err != nil {
		return agentdeployer.AgentInfo{}, fmt.Errorf("failed to create service logs dir: %w", err)
	}
	info.Logs.Folder.Local = dirPath

	info.Policy.Name = policy.Name
	info.Policy.ID = policy.ID

	// Copy all agent settings from the test configuration file
	info.Agent.AgentSettings = config.Agent.AgentSettings

	// If user is defined in the configuration file, it has preference
	// and it should not be overwritten by the value in the manifest
	if info.Agent.User == "" && agentManifest.Privileges.Root {
		info.Agent.User = "root"
	}

	return info, nil
}

func (r *runner) createServiceInfo() (servicedeployer.ServiceInfo, error) {
	var svcInfo servicedeployer.ServiceInfo
	svcInfo.Name = r.options.TestFolder.Package
	svcInfo.Logs.Folder.Local = r.locationManager.ServiceLogDir()
	svcInfo.Logs.Folder.Agent = ServiceLogsAgentDir
	svcInfo.Test.RunID = createTestRunID()

	if r.options.RunTearDown || r.options.RunTestsOnly {
		logger.Debug("Skip creating output directory")
	} else {
		outputDir, err := servicedeployer.CreateOutputDir(r.locationManager, svcInfo.Test.RunID)
		if err != nil {
			return servicedeployer.ServiceInfo{}, fmt.Errorf("could not create output dir for terraform deployer %w", err)
		}
		svcInfo.OutputDir = outputDir
	}

	svcInfo.Agent.Independent = false

	return svcInfo, nil
}

// TearDown method doesn't perform any global action as the "tear down" is executed per test case.
func (r *runner) TearDown(ctx context.Context) error {
	return nil
}

func (r *runner) tearDownTest(ctx context.Context) error {
	if r.options.DeferCleanup > 0 {
		logger.Debugf("waiting for %s before tearing down...", r.options.DeferCleanup)
		select {
		case <-time.After(r.options.DeferCleanup):
		case <-ctx.Done():
		}
	}

	// Avoid cancellations during cleanup.
	cleanupCtx := context.WithoutCancel(ctx)

	// This handler should be run before shutting down Elastic Agents (agent deployer)
	// or services that could run agents like Custom Agents (service deployer)
	// or Kind deployer.
	if r.resetAgentPolicyHandler != nil {
		if err := r.resetAgentPolicyHandler(cleanupCtx); err != nil {
			return err
		}
		r.resetAgentPolicyHandler = nil
	}

	// Shutting down the service should be run one of the first actions
	// to ensure that resources created by terraform are deleted even if other
	// errors fail.
	if r.shutdownServiceHandler != nil {
		if err := r.shutdownServiceHandler(cleanupCtx); err != nil {
			return err
		}
		r.shutdownServiceHandler = nil
	}

	if r.cleanTestScenarioHandler != nil {
		if err := r.cleanTestScenarioHandler(cleanupCtx); err != nil {
			return err
		}
		r.cleanTestScenarioHandler = nil
	}

	if r.resetAgentLogLevelHandler != nil {
		if err := r.resetAgentLogLevelHandler(cleanupCtx); err != nil {
			return err
		}
		r.resetAgentLogLevelHandler = nil
	}

	if r.removeAgentHandler != nil {
		if err := r.removeAgentHandler(cleanupCtx); err != nil {
			return err
		}
		r.removeAgentHandler = nil
	}

	if r.deleteTestPolicyHandler != nil {
		if err := r.deleteTestPolicyHandler(cleanupCtx); err != nil {
			return err
		}
		r.deleteTestPolicyHandler = nil
	}

	resourcesOptions := resourcesOptions{
		// Keep it installed only if we were running setup, or tests only.
		installedPackage: r.options.RunSetup || r.options.RunTestsOnly,
	}
	_, err := r.resourcesManager.ApplyCtx(cleanupCtx, r.resources(resourcesOptions))
	if err != nil {
		return err
	}

	if r.shutdownAgentHandler != nil {
		if err := r.shutdownAgentHandler(cleanupCtx); err != nil {
			return err
		}
		r.shutdownAgentHandler = nil
	}

	if r.wipeDataStreamHandler != nil {
		if err := r.wipeDataStreamHandler(cleanupCtx); err != nil {
			return err
		}
		r.wipeDataStreamHandler = nil
	}

	return nil
}

func (r *runner) newResult(name string) *testrunner.ResultComposer {
	return testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   TestType,
		Name:       name,
		Package:    r.options.TestFolder.Package,
		DataStream: r.options.TestFolder.DataStream,
	})
}

func (r *runner) initRun() error {
	var err error
	var found bool
	r.locationManager, err = locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("reading service logs directory failed: %w", err)
	}

	r.serviceStateFilePath = filepath.Join(testrunner.StateFolderPath(r.options.Profile.ProfilePath), testrunner.ServiceStateFileName)

	r.dataStreamPath, found, err = packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return fmt.Errorf("locating data stream root failed: %w", err)
	}
	if found {
		logger.Debugf("Running system tests for data stream %q", r.options.TestFolder.DataStream)
	} else {
		logger.Debug("Running system tests for package")
	}

	if r.options.API == nil {
		return errors.New("missing Elasticsearch client")
	}
	if r.options.KibanaClient == nil {
		return errors.New("missing Kibana client")
	}

	r.stackVersion, err = r.options.KibanaClient.Version()
	if err != nil {
		return fmt.Errorf("cannot request Kibana version: %w", err)
	}

	devDeployPath, err := servicedeployer.FindDevDeployPath(servicedeployer.FactoryOptions{
		PackageRootPath:    r.options.PackageRootPath,
		DataStreamRootPath: r.dataStreamPath,
		DevDeployDir:       DevDeployDir,
	})
	switch {
	case errors.Is(err, os.ErrNotExist):
		r.variants = r.selectVariants(nil)
	case err != nil:
		return fmt.Errorf("failed fo find service deploy path: %w", err)
	default:
		variantsFile, err := servicedeployer.ReadVariantsFile(devDeployPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("can't read service variant: %w", err)
		}
		r.variants = r.selectVariants(variantsFile)
	}
	if r.options.ServiceVariant != "" && len(r.variants) == 0 {
		return fmt.Errorf("not found variant definition %q", r.options.ServiceVariant)
	}

	if r.options.ConfigFilePath != "" {
		allCfgFiles, err := listConfigFiles(filepath.Dir(r.options.ConfigFilePath))
		if err != nil {
			return fmt.Errorf("failed listing test case config cfgFiles: %w", err)
		}
		baseFile := filepath.Base(r.options.ConfigFilePath)
		for _, cfg := range allCfgFiles {
			if cfg == baseFile {
				r.cfgFiles = append(r.cfgFiles, baseFile)
			}
		}
	} else {
		r.cfgFiles, err = listConfigFiles(r.options.TestFolder.Path)
		if err != nil {
			return fmt.Errorf("failed listing test case config cfgFiles: %w", err)
		}
	}

	return nil
}

func (r *runner) run(ctx context.Context) (results []testrunner.TestResult, err error) {
	result := r.newResult("(init)")
	if err = r.initRun(); err != nil {
		return result.WithError(err)
	}

	if _, err = os.Stat(r.serviceStateFilePath); !os.IsNotExist(err) {
		return result.WithError(fmt.Errorf("failed to run tests, required to tear down previous state run: %s exists", r.serviceStateFilePath))
	}

	startTesting := time.Now()
	for _, cfgFile := range r.cfgFiles {
		for _, variantName := range r.variants {
			partial, err := r.runTestPerVariant(ctx, result, cfgFile, variantName)
			results = append(results, partial...)
			if err != nil {
				return results, err
			}
		}
	}

	tempDir, err := os.MkdirTemp("", "test-system-")
	if err != nil {
		return nil, fmt.Errorf("can't create temporal directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	stackConfig, err := stack.LoadConfig(r.options.Profile)
	if err != nil {
		return nil, err
	}

	provider, err := stack.BuildProvider(stackConfig.Provider, r.options.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to build stack provider: %w", err)
	}

	dumpOptions := stack.DumpOptions{
		Output:  tempDir,
		Profile: r.options.Profile,
	}
	dump, err := provider.Dump(context.WithoutCancel(ctx), dumpOptions)
	if err != nil {
		return nil, fmt.Errorf("dump failed: %w", err)
	}

	logResults, err := r.checkAgentLogs(dump, startTesting, errorPatterns)
	if err != nil {
		return result.WithError(err)
	}
	results = append(results, logResults...)

	return results, nil
}

func (r *runner) runTestPerVariant(ctx context.Context, result *testrunner.ResultComposer, cfgFile, variantName string) ([]testrunner.TestResult, error) {
	svcInfo, err := r.createServiceInfo()
	if err != nil {
		return result.WithError(err)
	}

	configFile := filepath.Join(r.options.TestFolder.Path, cfgFile)
	testConfig, err := newConfig(configFile, svcInfo, variantName)
	if err != nil {
		return nil, fmt.Errorf("unable to load system test case file '%s': %w", configFile, err)
	}
	logger.Debugf("Using config: %q", testConfig.Name())

	partial, err := r.runTest(ctx, testConfig, svcInfo)

	tdErr := r.tearDownTest(ctx)
	if err != nil {
		return partial, err
	}
	if tdErr != nil {
		return partial, fmt.Errorf("failed to tear down runner: %w", tdErr)
	}
	return partial, nil
}

func createTestRunID() string {
	return fmt.Sprintf("%d", rand.Intn(testRunMaxID-testRunMinID)+testRunMinID)
}

func (r *runner) isSyntheticsEnabled(ctx context.Context, dataStream, componentTemplatePackage string) (bool, error) {
	resp, err := r.options.API.Cluster.GetComponentTemplate(
		r.options.API.Cluster.GetComponentTemplate.WithContext(ctx),
		r.options.API.Cluster.GetComponentTemplate.WithName(componentTemplatePackage),
	)
	if err != nil {
		return false, fmt.Errorf("could not get component template %s from data stream %s: %w", componentTemplatePackage, dataStream, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// @package component template doesn't exist before 8.2. On these versions synthetics was not supported
		// in any case, so just return false.
		logger.Debugf("no component template %s found for data stream %s", componentTemplatePackage, dataStream)
		return false, nil
	}
	if resp.IsError() {
		return false, fmt.Errorf("could not get component template %s for data stream %s: %s", componentTemplatePackage, dataStream, resp.String())
	}

	var results struct {
		ComponentTemplates []struct {
			Name              string `json:"name"`
			ComponentTemplate struct {
				Template struct {
					Mappings struct {
						Source *struct {
							Mode string `json:"mode"`
						} `json:"_source,omitempty"`
					} `json:"mappings"`
				} `json:"template"`
			} `json:"component_template"`
		} `json:"component_templates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return false, fmt.Errorf("could not decode search results response: %w", err)
	}

	if len(results.ComponentTemplates) == 0 {
		logger.Debugf("no component template %s found for data stream %s", componentTemplatePackage, dataStream)
		return false, nil
	}
	if len(results.ComponentTemplates) != 1 {
		return false, fmt.Errorf("ambiguous response, expected one component template for %s, found %d", componentTemplatePackage, len(results.ComponentTemplates))
	}

	template := results.ComponentTemplates[0]

	if template.ComponentTemplate.Template.Mappings.Source == nil {
		return false, nil
	}

	return template.ComponentTemplate.Template.Mappings.Source.Mode == "synthetic", nil
}

type hits struct {
	Source        []common.MapStr `json:"_source"`
	Fields        []common.MapStr `json:"fields"`
	IgnoredFields []string
	DegradedDocs  []common.MapStr
}

func (h hits) getDocs(syntheticsEnabled bool) []common.MapStr {
	if syntheticsEnabled {
		return h.Fields
	}
	return h.Source
}

func (h hits) size() int {
	return len(h.Source)
}

func (r *runner) getDocs(ctx context.Context, dataStream string) (*hits, error) {
	resp, err := r.options.API.Search(
		r.options.API.Search.WithContext(ctx),
		r.options.API.Search.WithIndex(dataStream),
		r.options.API.Search.WithSort("@timestamp:asc"),
		r.options.API.Search.WithSize(elasticsearchQuerySize),
		r.options.API.Search.WithSource("true"),
		r.options.API.Search.WithBody(strings.NewReader(checkFieldsBody)),
		r.options.API.Search.WithIgnoreUnavailable(true),
	)
	if err != nil {
		return nil, fmt.Errorf("could not search data stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable && strings.Contains(resp.String(), "no_shard_available_action_exception") {
		// Index is being created, but no shards are available yet.
		// See https://github.com/elastic/elasticsearch/issues/65846
		return &hits{}, nil
	}
	if resp.IsError() {
		return nil, fmt.Errorf("failed to search docs for data stream %s: %s", dataStream, resp.String())
	}

	var results struct {
		Hits struct {
			Total struct {
				Value int
			}
			Hits []struct {
				Source common.MapStr `json:"_source"`
				Fields common.MapStr `json:"fields"`
			}
		}
		Aggregations struct {
			AllIgnored struct {
				DocCount      int `json:"doc_count"`
				IgnoredFields struct {
					Buckets []struct {
						Key string `json:"key"`
					} `json:"buckets"`
				} `json:"ignored_fields"`
				IgnoredDocs struct {
					Hits struct {
						Hits []common.MapStr `json:"hits"`
					} `json:"hits"`
				} `json:"ignored_docs"`
			} `json:"all_ignored"`
		} `json:"aggregations"`
		Error *struct {
			Type   string
			Reason string
		}
		Status int
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("could not decode search results response: %w", err)
	}

	numHits := results.Hits.Total.Value
	if results.Error != nil {
		logger.Debugf("found %d hits in %s data stream: %s: %s Status=%d",
			numHits, dataStream, results.Error.Type, results.Error.Reason, results.Status)
	} else {
		logger.Debugf("found %d hits in %s data stream", numHits, dataStream)
	}

	var hits hits
	for _, hit := range results.Hits.Hits {
		hits.Source = append(hits.Source, hit.Source)
		hits.Fields = append(hits.Fields, hit.Fields)
	}
	for _, bucket := range results.Aggregations.AllIgnored.IgnoredFields.Buckets {
		hits.IgnoredFields = append(hits.IgnoredFields, bucket.Key)
	}
	hits.DegradedDocs = results.Aggregations.AllIgnored.IgnoredDocs.Hits.Hits

	return &hits, nil
}

type scenarioTest struct {
	dataStream         string
	policyTemplateName string
	pkgManifest        *packages.PackageManifest
	dataStreamManifest *packages.DataStreamManifest
	kibanaDataStream   kibana.PackageDataStream
	syntheticEnabled   bool
	docs               []common.MapStr
	ignoredFields      []string
	degradedDocs       []common.MapStr
	agent              agentdeployer.DeployedAgent
	startTestTime      time.Time
}

func (r *runner) shouldCreateNewAgentPolicyForTest() bool {
	if r.options.RunTestsOnly {
		// always that --no-provision is set, it should create new Agent Policies.
		return true
	}
	if !r.options.RunIndependentElasticAgent {
		// keep same behaviour as previously when Elastic Agent of the stack is used.
		return false
	}
	// No need to create new Agent Policies for these stages
	if r.options.RunSetup || r.options.RunTearDown {
		return false
	}
	return true
}

func (r *runner) deleteDataStream(ctx context.Context, dataStream string) error {
	resp, err := r.options.API.Indices.DeleteDataStream([]string{dataStream},
		r.options.API.Indices.DeleteDataStream.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to delete data stream %s: %w", dataStream, err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("could not get delete data stream %s: %s", dataStream, resp.String())
	}
	return nil
}

func (r *runner) prepareScenario(ctx context.Context, config *testConfig, svcInfo servicedeployer.ServiceInfo) (*scenarioTest, error) {
	serviceOptions := r.createServiceOptions(config.ServiceVariantName)

	var err error
	var serviceStateData ServiceState
	if r.options.RunSetup {
		err = r.createServiceStateDir()
		if err != nil {
			return nil, fmt.Errorf("failed to create setup services dir: %w", err)
		}
	}
	scenario := scenarioTest{}

	if r.options.RunTearDown || r.options.RunTestsOnly {
		serviceStateData, err = r.readServiceStateData()
		if err != nil {
			return nil, fmt.Errorf("failed to read service setup data: %w", err)
		}
	}

	scenario.pkgManifest, err = packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed: %w", err)
	}

	scenario.dataStreamManifest, err = packages.ReadDataStreamManifest(filepath.Join(r.dataStreamPath, packages.DataStreamManifestFile))
	if err != nil {
		return nil, fmt.Errorf("reading data stream manifest failed: %w", err)
	}

	// Temporarily until independent Elastic Agents are enabled by default,
	// enable independent Elastic Agents if package defines that requires root privileges
	if pkg, ds := scenario.pkgManifest, scenario.dataStreamManifest; pkg.Agent.Privileges.Root || (ds != nil && ds.Agent.Privileges.Root) {
		r.options.RunIndependentElasticAgent = true
	}

	// If the environment variable is present, it always has preference over the root
	// privileges value (if any) defined in the manifest file
	v, ok := os.LookupEnv(enableIndependentAgents)
	if ok {
		r.options.RunIndependentElasticAgent = strings.ToLower(v) == "true"
	}
	serviceOptions.DeployIndependentAgent = r.options.RunIndependentElasticAgent

	policyTemplateName := config.PolicyTemplate
	if policyTemplateName == "" {
		policyTemplateName, err = findPolicyTemplateForInput(*scenario.pkgManifest, *scenario.dataStreamManifest, config.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to determine the associated policy_template: %w", err)
		}
	}
	scenario.policyTemplateName = policyTemplateName

	policyTemplate, err := selectPolicyTemplateByName(scenario.pkgManifest.PolicyTemplates, scenario.policyTemplateName)
	if err != nil {
		return nil, fmt.Errorf("failed to find the selected policy_template: %w", err)
	}

	// Configure package (single data stream) via Fleet APIs.
	var policyToTest, policyToEnroll *kibana.Policy
	if r.options.RunTearDown || r.options.RunTestsOnly {
		policyToTest = &serviceStateData.CurrentPolicy
		policyToEnroll = &serviceStateData.EnrollPolicy
		logger.Debugf("Got policy from file: %q - %q", policyToTest.Name, policyToTest.ID)
	} else {
		// Create two different policies, one for enrolling the agent and the other for testing.
		// This allows us to ensure that the Agent Policy used for testing is
		// assigned to the agent with all the required changes (e.g. Package DataStream)
		logger.Debug("creating test policies...")
		testTime := time.Now().Format("20060102T15:04:05Z")

		policyEnroll := kibana.Policy{
			Name:        fmt.Sprintf("ep-test-system-enroll-%s-%s-%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream, testTime),
			Description: fmt.Sprintf("test policy created by elastic-package to enroll agent for data stream %s/%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream),
			Namespace:   "ep",
		}

		policyTest := kibana.Policy{
			Name:        fmt.Sprintf("ep-test-system-%s-%s-%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream, testTime),
			Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream),
			Namespace:   "ep",
		}
		// Assign the data_output_id to the agent policy to configure the output to logstash. The value is inferred from stack/_static/kibana.yml.tmpl
		if r.options.Profile.Config("stack.logstash_enabled", "false") == "true" {
			policyTest.DataOutputID = "fleet-logstash-output"
		}
		policyToTest, err = r.options.KibanaClient.CreatePolicy(ctx, policyTest)
		if err != nil {
			return nil, fmt.Errorf("could not create test policy: %w", err)
		}
		policyToEnroll, err = r.options.KibanaClient.CreatePolicy(ctx, policyEnroll)
		if err != nil {
			return nil, fmt.Errorf("could not create test policy: %w", err)
		}
	}
	r.deleteTestPolicyHandler = func(ctx context.Context) error {
		if r.options.RunTestsOnly {
			return nil
		}
		logger.Debug("deleting test policies...")
		if err := r.options.KibanaClient.DeletePolicy(ctx, policyToTest.ID); err != nil {
			return fmt.Errorf("error cleaning up test policy: %w", err)
		}
		if err := r.options.KibanaClient.DeletePolicy(ctx, policyToEnroll.ID); err != nil {
			return fmt.Errorf("error cleaning up test policy: %w", err)
		}
		return nil
	}

	// policyToEnroll is used in both independent agents and agents created by servicedeployer (custom or kubernetes agents)
	policy := policyToEnroll
	if r.options.RunTearDown || r.options.RunTestsOnly {
		// required in order to be able select the right agent in `checkEnrolledAgents` when
		// using independent agents or custom/kubernetes agents since policy data is set into `agentInfo` variable`
		policy = policyToTest
	}

	agentDeployed, agentInfo, err := r.setupAgent(ctx, config, serviceStateData, policy, scenario.pkgManifest.Agent)
	if err != nil {
		return nil, err
	}

	scenario.agent = agentDeployed

	service, svcInfo, err := r.setupService(ctx, config, serviceOptions, svcInfo, agentInfo, agentDeployed, policy, serviceStateData)
	if errors.Is(err, os.ErrNotExist) {
		logger.Debugf("No service deployer defined for this test")
	} else if err != nil {
		return nil, err
	}

	// Reload test config with ctx variable substitution.
	config, err = newConfig(config.Path, svcInfo, serviceOptions.Variant)
	if err != nil {
		return nil, fmt.Errorf("unable to reload system test case configuration: %w", err)
	}

	// store the time just before adding the Test Policy, this time will be used to check
	// the agent logs from that time onwards to avoid possible previous errors present in logs
	scenario.startTestTime = time.Now()
	suffixDatastream := svcInfo.Test.RunID

	logger.Debug("adding package data stream to test policy...")
	policyTesting := kibana.Policy{
		Name:        fmt.Sprintf("ep-one-test-system-%s-%s-%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream, scenario.startTestTime.Format("20060102T15:04:05Z")),
		Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.options.TestFolder.Package, r.options.TestFolder.DataStream),
		Namespace:   createTestRunID(),
	}
	policyToAssignDatastreamTests := policyToTest
	if r.shouldCreateNewAgentPolicyForTest() {
		policyToAssignDatastreamTests, err = r.options.KibanaClient.CreatePolicy(ctx, policyTesting)
		if err != nil {
			return nil, fmt.Errorf("could not create test policy: %w", err)
		}
		suffixDatastream = policyTesting.Namespace
	}
	ds := createPackageDatastream(*policyToAssignDatastreamTests, *scenario.pkgManifest, policyTemplate, *scenario.dataStreamManifest, *config, suffixDatastream)
	if r.options.RunTearDown {
		logger.Debug("Skip adding data stream config to policy")
	} else {
		if err := r.options.KibanaClient.AddPackageDataStreamToPolicy(ctx, ds); err != nil {
			return nil, fmt.Errorf("could not add data stream config to policy: %w", err)
		}
	}
	scenario.kibanaDataStream = ds

	// Delete old data
	logger.Debug("deleting old data in data stream...")

	// Input packages can set `data_stream.dataset` by convention to customize the dataset.
	dataStreamDataset := ds.Inputs[0].Streams[0].DataStream.Dataset
	if scenario.pkgManifest.Type == "input" {
		v, _ := config.Vars.GetValue("data_stream.dataset")
		if dataset, ok := v.(string); ok && dataset != "" {
			dataStreamDataset = dataset
		}
	}
	scenario.dataStream = fmt.Sprintf(
		"%s-%s-%s",
		ds.Inputs[0].Streams[0].DataStream.Type,
		dataStreamDataset,
		ds.Namespace,
	)
	componentTemplatePackage := fmt.Sprintf(
		"%s-%s@package",
		ds.Inputs[0].Streams[0].DataStream.Type,
		dataStreamDataset,
	)

	r.wipeDataStreamHandler = func(ctx context.Context) error {
		if r.options.RunTestsOnly {
			return nil
		}
		logger.Debugf("deleting data in data stream...")
		if err := deleteDataStreamDocs(ctx, r.options.API, scenario.dataStream); err != nil {
			return fmt.Errorf("error deleting data in data stream: %w", err)
		}
		return nil
	}

	r.cleanTestScenarioHandler = func(ctx context.Context) error {
		if !r.shouldCreateNewAgentPolicyForTest() {
			return nil
		}

		logger.Debug("Deleting test policy...")
		err = r.options.KibanaClient.DeletePolicy(ctx, policyToAssignDatastreamTests.ID)
		if err != nil {
			return fmt.Errorf("failed to delete policy %s: %w", policyToAssignDatastreamTests.Name, err)
		}
		logger.Debug("Deleting data stream for testing")
		r.deleteDataStream(ctx, scenario.dataStream)
		if err != nil {
			return fmt.Errorf("failed to delete data stream %s: %w", scenario.dataStream, err)
		}
		return nil
	}

	if r.options.RunTearDown {
		logger.Debugf("Skipped deleting old data in data stream %q", scenario.dataStream)
	} else {
		err := r.deleteOldDocumentsDataStreamAndWait(ctx, scenario.dataStream)
		if err != nil {
			return nil, err
		}
	}

	// FIXME: running per stages does not work when multiple agents are created
	var origPolicy kibana.Policy
	agents, err := checkEnrolledAgents(ctx, r.options.KibanaClient, agentInfo, svcInfo, r.options.RunIndependentElasticAgent)
	if err != nil {
		return nil, fmt.Errorf("can't check enrolled agents: %w", err)
	}
	agent := agents[0]
	logger.Debugf("Selected enrolled agent %q", agent.ID)

	r.removeAgentHandler = func(ctx context.Context) error {
		if r.options.RunTestsOnly {
			return nil
		}
		// When not using independent agents, service deployers like kubernetes or custom agents create new Elastic Agent
		if !r.options.RunIndependentElasticAgent && !svcInfo.Agent.Independent {
			return nil
		}
		logger.Debug("removing agent...")
		err := r.options.KibanaClient.RemoveAgent(ctx, agent)
		if err != nil {
			return fmt.Errorf("failed to remove agent %q: %w", agent.ID, err)
		}
		return nil
	}

	if r.options.RunTearDown {
		origPolicy = serviceStateData.OrigPolicy
		logger.Debugf("Got orig policy from file: %q - %q", origPolicy.Name, origPolicy.ID)
	} else {
		// Store previous agent policy assigned to the agent
		origPolicy = kibana.Policy{
			ID:       agent.PolicyID,
			Revision: agent.PolicyRevision,
		}
	}

	r.resetAgentPolicyHandler = func(ctx context.Context) error {
		if !r.options.RunSetup {
			// it should be kept the same policy just when system tests are
			// triggered with the flags for running setup stage (--setup)
			logger.Debug("reassigning original policy back to agent...")
			if err := r.options.KibanaClient.AssignPolicyToAgent(ctx, agent, origPolicy); err != nil {
				return fmt.Errorf("error reassigning original policy to agent: %w", err)
			}
		}
		return nil
	}

	origAgent := agent
	origLogLevel := ""
	if r.options.RunTearDown {
		logger.Debug("Skip assiging log level debug to agent")
		origLogLevel = serviceStateData.Agent.LocalMetadata.Elastic.Agent.LogLevel
	} else {
		logger.Debug("Set Debug log level to agent")
		origLogLevel = agent.LocalMetadata.Elastic.Agent.LogLevel
		err = r.options.KibanaClient.SetAgentLogLevel(ctx, agent.ID, "debug")
		if err != nil {
			return nil, fmt.Errorf("error setting log level debug for agent %s: %w", agent.ID, err)
		}
	}
	r.resetAgentLogLevelHandler = func(ctx context.Context) error {
		if r.options.RunTestsOnly {
			return nil
		}
		logger.Debugf("reassigning original log level %q back to agent...", origLogLevel)

		if err := r.options.KibanaClient.SetAgentLogLevel(ctx, agent.ID, origLogLevel); err != nil {
			return fmt.Errorf("error reassigning original log level to agent: %w", err)
		}
		return nil
	}

	if r.options.RunTearDown {
		logger.Debug("Skip assiging package data stream to agent")
	} else {
		policyWithDataStream, err := r.options.KibanaClient.GetPolicy(ctx, policyToAssignDatastreamTests.ID)
		if err != nil {
			return nil, fmt.Errorf("could not read the policy with data stream: %w", err)
		}

		logger.Debug("assigning package data stream to agent...")
		if err := r.options.KibanaClient.AssignPolicyToAgent(ctx, agent, *policyWithDataStream); err != nil {
			return nil, fmt.Errorf("could not assign policy to agent: %w", err)
		}
	}

	// Signal to the service that the agent is ready (policy is assigned).
	if service != nil && config.ServiceNotifySignal != "" {
		if err = service.Signal(ctx, config.ServiceNotifySignal); err != nil {
			return nil, fmt.Errorf("failed to notify test service: %w", err)
		}
	}

	if r.options.RunTearDown {
		return &scenario, nil
	}

	// Use custom timeout if the service can't collect data immediately.
	waitForDataTimeout := waitForDataDefaultTimeout
	if config.WaitForDataTimeout > 0 {
		waitForDataTimeout = config.WaitForDataTimeout
	}

	// (TODO in future) Optionally exercise service to generate load.
	logger.Debugf("checking for expected data in data stream (%s)...", waitForDataTimeout)
	var hits *hits
	oldHits := 0
	passed, waitErr := wait.UntilTrue(ctx, func(ctx context.Context) (bool, error) {
		var err error
		hits, err = r.getDocs(ctx, scenario.dataStream)
		if err != nil {
			return false, err
		}

		if config.Assert.HitCount > 0 {
			if hits.size() < config.Assert.HitCount {
				return false, nil
			}

			ret := hits.size() == oldHits
			if !ret {
				oldHits = hits.size()
				time.Sleep(4 * time.Second)
			}

			return ret, nil
		}

		return hits.size() > 0, nil
	}, 1*time.Second, waitForDataTimeout)

	if service != nil && config.Service != "" && !config.IgnoreServiceError {
		exited, code, err := service.ExitCode(ctx, config.Service)
		if err != nil && !errors.Is(err, servicedeployer.ErrNotSupported) {
			return nil, err
		}
		if exited && code > 0 {
			return nil, testrunner.ErrTestCaseFailed{Reason: fmt.Sprintf("the test service %s unexpectedly exited with code %d", config.Service, code)}
		}
	}

	if waitErr != nil {
		return nil, waitErr
	}

	if !passed {
		return nil, testrunner.ErrTestCaseFailed{Reason: fmt.Sprintf("could not find hits in %s data stream", scenario.dataStream)}
	}

	logger.Debugf("check whether or not synthetics is enabled (component template %s)...", componentTemplatePackage)
	scenario.syntheticEnabled, err = r.isSyntheticsEnabled(ctx, scenario.dataStream, componentTemplatePackage)
	if err != nil {
		return nil, fmt.Errorf("failed to check if synthetic source is enabled: %w", err)
	}
	logger.Debugf("data stream %s has synthetics enabled: %t", scenario.dataStream, scenario.syntheticEnabled)

	scenario.docs = hits.getDocs(scenario.syntheticEnabled)
	scenario.ignoredFields = hits.IgnoredFields
	scenario.degradedDocs = hits.DegradedDocs

	if r.options.RunSetup {
		opts := scenarioStateOpts{
			origPolicy:    &origPolicy,
			enrollPolicy:  policyToEnroll,
			currentPolicy: policyToTest,
			config:        config,
			agent:         origAgent,
			agentInfo:     agentInfo,
			svcInfo:       svcInfo,
		}
		err = r.writeScenarioState(opts)
		if err != nil {
			return nil, err
		}
	}

	return &scenario, nil
}

func (r *runner) setupService(ctx context.Context, config *testConfig, serviceOptions servicedeployer.FactoryOptions, svcInfo servicedeployer.ServiceInfo, agentInfo agentdeployer.AgentInfo, agentDeployed agentdeployer.DeployedAgent, policy *kibana.Policy, state ServiceState) (servicedeployer.DeployedService, servicedeployer.ServiceInfo, error) {
	logger.Debug("setting up service...")
	if r.options.RunTearDown || r.options.RunTestsOnly {
		svcInfo.Test.RunID = state.ServiceRunID
		svcInfo.OutputDir = state.ServiceOutputDir
	}

	// By default using agent running in the Elastic stack
	svcInfo.AgentNetworkName = stack.Network(r.options.Profile)
	if agentDeployed != nil {
		svcInfo.AgentNetworkName = agentInfo.NetworkName
	}

	// Set the right folder for logs execpt for custom agents that are still deployed using "servicedeployer"
	if r.options.RunIndependentElasticAgent && agentDeployed != nil {
		svcInfo.Logs.Folder.Local = agentInfo.Logs.Folder.Local
	}

	// In case of custom or kubernetes agents (servicedeployer) it is needed also the Agent Policy created
	// for each test execution
	serviceOptions.PolicyName = policy.Name

	if config.Service != "" {
		svcInfo.Name = config.Service
	}

	serviceDeployer, err := servicedeployer.Factory(serviceOptions)
	if err != nil {
		return nil, svcInfo, fmt.Errorf("could not create service runner: %w", err)
	}

	service, err := serviceDeployer.SetUp(ctx, svcInfo)
	if err != nil {
		return nil, svcInfo, fmt.Errorf("could not setup service: %w", err)
	}

	r.shutdownServiceHandler = func(ctx context.Context) error {
		if r.options.RunTestsOnly {
			return nil
		}
		logger.Debug("tearing down service...")
		if err := service.TearDown(ctx); err != nil {
			return fmt.Errorf("error tearing down service: %w", err)
		}

		return nil
	}
	return service, service.Info(), nil
}

func (r *runner) setupAgent(ctx context.Context, config *testConfig, state ServiceState, policy *kibana.Policy, agentManifest packages.Agent) (agentdeployer.DeployedAgent, agentdeployer.AgentInfo, error) {
	if !r.options.RunIndependentElasticAgent {
		return nil, agentdeployer.AgentInfo{}, nil
	}
	agentRunID := createTestRunID()
	if r.options.RunTearDown || r.options.RunTestsOnly {
		agentRunID = state.AgentRunID
	}
	logger.Warn("setting up agent (technical preview)...")
	agentInfo, err := r.createAgentInfo(policy, config, agentRunID, agentManifest)
	if err != nil {
		return nil, agentdeployer.AgentInfo{}, err
	}

	agentOptions := r.createAgentOptions(agentInfo.Policy.Name)
	agentDeployer, err := agentdeployer.Factory(agentOptions)
	if err != nil {
		return nil, agentInfo, fmt.Errorf("could not create agent runner: %w", err)
	}
	if agentDeployer == nil {
		logger.Debug("Not found agent deployer. Agent will be created along with the service.")
		return nil, agentInfo, nil
	}

	agentDeployed, err := agentDeployer.SetUp(ctx, agentInfo)
	if err != nil {
		return nil, agentInfo, fmt.Errorf("could not setup agent: %w", err)
	}
	r.shutdownAgentHandler = func(ctx context.Context) error {
		if r.options.RunTestsOnly {
			return nil
		}
		if agentDeployer == nil {
			return nil
		}
		logger.Debug("tearing down agent...")
		if err := agentDeployed.TearDown(ctx); err != nil {
			return fmt.Errorf("error tearing down agent: %w", err)
		}

		return nil
	}
	return agentDeployed, agentDeployed.Info(), nil
}

func (r *runner) removeServiceStateFile() error {
	err := os.Remove(r.serviceStateFilePath)
	if err != nil {
		return fmt.Errorf("failed to remove file %q: %w", r.serviceStateFilePath, err)
	}
	return nil
}

func (r *runner) createServiceStateDir() error {
	dirPath := filepath.Dir(r.serviceStateFilePath)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("mkdir failed (path: %s): %w", dirPath, err)
	}
	return nil
}

func (r *runner) readServiceStateData() (ServiceState, error) {
	var setupData ServiceState
	logger.Debugf("Reading test config from file %s", r.serviceStateFilePath)
	contents, err := os.ReadFile(r.serviceStateFilePath)
	if err != nil {
		return setupData, fmt.Errorf("failed to read test config %q: %w", r.serviceStateFilePath, err)
	}
	err = json.Unmarshal(contents, &setupData)
	if err != nil {
		return setupData, fmt.Errorf("failed to decode service options %q: %w", r.serviceStateFilePath, err)
	}
	return setupData, nil
}

type ServiceState struct {
	OrigPolicy       kibana.Policy `json:"orig_policy"`
	EnrollPolicy     kibana.Policy `json:"enroll_policy"`
	CurrentPolicy    kibana.Policy `json:"current_policy"`
	Agent            kibana.Agent  `json:"agent"`
	ConfigFilePath   string        `json:"config_file_path"`
	VariantName      string        `json:"variant_name"`
	ServiceRunID     string        `json:"service_info_run_id"`
	AgentRunID       string        `json:"agent_info_run_id"`
	ServiceOutputDir string        `json:"service_output_dir"`
}

type scenarioStateOpts struct {
	currentPolicy *kibana.Policy
	enrollPolicy  *kibana.Policy
	origPolicy    *kibana.Policy
	config        *testConfig
	agent         kibana.Agent
	agentInfo     agentdeployer.AgentInfo
	svcInfo       servicedeployer.ServiceInfo
}

func (r *runner) writeScenarioState(opts scenarioStateOpts) error {
	data := ServiceState{
		OrigPolicy:       *opts.origPolicy,
		EnrollPolicy:     *opts.enrollPolicy,
		CurrentPolicy:    *opts.currentPolicy,
		Agent:            opts.agent,
		ConfigFilePath:   opts.config.Path,
		VariantName:      opts.config.ServiceVariantName,
		ServiceRunID:     opts.svcInfo.Test.RunID,
		AgentRunID:       opts.agentInfo.Test.RunID,
		ServiceOutputDir: opts.svcInfo.OutputDir,
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshall service setup data: %w", err)
	}

	err = os.WriteFile(r.serviceStateFilePath, dataBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write service setup JSON: %w", err)
	}
	return nil
}

func (r *runner) deleteOldDocumentsDataStreamAndWait(ctx context.Context, dataStream string) error {
	logger.Debugf("Delete previous documents in data stream %q", dataStream)
	if err := deleteDataStreamDocs(ctx, r.options.API, dataStream); err != nil {
		return fmt.Errorf("error deleting old data in data stream: %s: %w", dataStream, err)
	}
	cleared, err := wait.UntilTrue(ctx, func(ctx context.Context) (bool, error) {
		hits, err := r.getDocs(ctx, dataStream)
		if err != nil {
			return false, err
		}
		return hits.size() == 0, nil
	}, 1*time.Second, 2*time.Minute)
	if err != nil || !cleared {
		if err == nil {
			err = errors.New("unable to clear previous data")
		}
		return err
	}
	return nil
}

func (r *runner) validateTestScenario(ctx context.Context, result *testrunner.ResultComposer, scenario *scenarioTest, config *testConfig) ([]testrunner.TestResult, error) {
	// Validate fields in docs
	// when reroute processors are used, expectedDatasets should be set depends on the processor config
	var expectedDatasets []string
	for _, pipeline := range r.pipelines {
		var esIngestPipeline map[string]any
		err := yaml.Unmarshal(pipeline.Content, &esIngestPipeline)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling ingest pipeline content failed: %w", err)
		}
		processors, _ := esIngestPipeline["processors"].([]any)
		for _, p := range processors {
			processor, ok := p.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unexpected processor %+v", p)
			}
			if reroute, ok := processor["reroute"]; ok {
				if rerouteP, ok := reroute.(ingest.RerouteProcessor); ok {
					expectedDatasets = append(expectedDatasets, rerouteP.Dataset...)
				}
			}
		}
	}

	if expectedDatasets == nil {
		var expectedDataset string
		if ds := r.options.TestFolder.DataStream; ds != "" {
			expectedDataset = getDataStreamDataset(*scenario.pkgManifest, *scenario.dataStreamManifest)
		} else {
			expectedDataset = scenario.pkgManifest.Name + "." + scenario.policyTemplateName
		}
		expectedDatasets = []string{expectedDataset}
	}
	if scenario.pkgManifest.Type == "input" {
		v, _ := config.Vars.GetValue("data_stream.dataset")
		if dataset, ok := v.(string); ok && dataset != "" {
			expectedDatasets = append(expectedDatasets, dataset)
		}
	}

	fieldsValidator, err := fields.CreateValidatorForDirectory(r.dataStreamPath,
		fields.WithSpecVersion(scenario.pkgManifest.SpecVersion),
		fields.WithNumericKeywordFields(config.NumericKeywordFields),
		fields.WithExpectedDatasets(expectedDatasets),
		fields.WithEnabledImportAllECSSChema(true),
		fields.WithDisableNormalization(scenario.syntheticEnabled),
	)
	if err != nil {
		return result.WithError(fmt.Errorf("creating fields validator for data stream failed (path: %s): %w", r.dataStreamPath, err))
	}
	if err := validateFields(scenario.docs, fieldsValidator, scenario.dataStream); err != nil {
		return result.WithError(err)
	}

	err = validateIgnoredFields(r.stackVersion.Number, scenario, config)
	if err != nil {
		return result.WithError(err)
	}

	docs := scenario.docs
	if scenario.syntheticEnabled {
		docs, err = fieldsValidator.SanitizeSyntheticSourceDocs(scenario.docs)
		if err != nil {
			return result.WithError(fmt.Errorf("failed to sanitize synthetic source docs: %w", err))
		}
	}

	specVersion, err := semver.NewVersion(scenario.pkgManifest.SpecVersion)
	if err != nil {
		return result.WithError(fmt.Errorf("failed to parse format version %q: %w", scenario.pkgManifest.SpecVersion, err))
	}

	// Write sample events file from first doc, if requested
	if err := r.generateTestResult(docs, *specVersion); err != nil {
		return result.WithError(err)
	}

	// Check Hit Count within docs, if 0 then it has not been specified
	if assertionPass, message := assertHitCount(config.Assert.HitCount, docs); !assertionPass {
		result.FailureMsg = message
	}

	// Check transforms if present
	if err := r.checkTransforms(ctx, config, scenario.pkgManifest, scenario.kibanaDataStream, scenario.dataStream); err != nil {
		return result.WithError(err)
	}

	if scenario.agent != nil {
		logResults, err := r.checkNewAgentLogs(ctx, scenario.agent, scenario.startTestTime, errorPatterns)
		if err != nil {
			return result.WithError(err)
		}
		if len(logResults) > 0 {
			return logResults, nil
		}
	}

	return result.WithSuccess()
}

func (r *runner) runTest(ctx context.Context, config *testConfig, svcInfo servicedeployer.ServiceInfo) ([]testrunner.TestResult, error) {
	result := r.newResult(config.Name())

	if config.Skip != nil {
		logger.Warnf("skipping %s test for %s/%s: %s (details: %s)",
			TestType, r.options.TestFolder.Package, r.options.TestFolder.DataStream,
			config.Skip.Reason, config.Skip.Link.String())
		return result.WithSkip(config.Skip)
	}

	logger.Debugf("running test with configuration '%s'", config.Name())

	scenario, err := r.prepareScenario(ctx, config, svcInfo)
	if err != nil {
		return result.WithError(err)
	}

	return r.validateTestScenario(ctx, result, scenario, config)
}

func checkEnrolledAgents(ctx context.Context, client *kibana.Client, agentInfo agentdeployer.AgentInfo, svcInfo servicedeployer.ServiceInfo, runIndependentElasticAgent bool) ([]kibana.Agent, error) {
	var agents []kibana.Agent

	enrolled, err := wait.UntilTrue(ctx, func(ctx context.Context) (bool, error) {
		allAgents, err := client.ListAgents(ctx)
		if err != nil {
			return false, fmt.Errorf("could not list agents: %w", err)
		}

		if runIndependentElasticAgent {
			agents = filterIndependentAgents(allAgents, agentInfo)
		} else {
			agents = filterAgents(allAgents, svcInfo)
		}
		logger.Debugf("found %d enrolled agent(s)", len(agents))
		if len(agents) == 0 {
			return false, nil // selected agents are unavailable yet
		}
		return true, nil
	}, 1*time.Second, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("agent enrollment failed: %w", err)
	}
	if !enrolled {
		return nil, errors.New("no agent enrolled in time")
	}
	return agents, nil
}

func createPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	ds packages.DataStreamManifest,
	config testConfig,
	suffix string,
) kibana.PackageDataStream {
	if pkg.Type == "input" {
		return createInputPackageDatastream(kibanaPolicy, pkg, policyTemplate, config, suffix)
	}
	return createIntegrationPackageDatastream(kibanaPolicy, pkg, policyTemplate, ds, config, suffix)
}

func createIntegrationPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	ds packages.DataStreamManifest,
	config testConfig,
	suffix string,
) kibana.PackageDataStream {
	r := kibana.PackageDataStream{
		Name:      fmt.Sprintf("%s-%s-%s", pkg.Name, ds.Name, suffix),
		Namespace: kibanaPolicy.Namespace,
		PolicyID:  kibanaPolicy.ID,
		Enabled:   true,
		Inputs: []kibana.Input{
			{
				PolicyTemplate: policyTemplate.Name,
				Enabled:        true,
			},
		},
	}
	r.Package.Name = pkg.Name
	r.Package.Title = pkg.Title
	r.Package.Version = pkg.Version

	stream := ds.Streams[getDataStreamIndex(config.Input, ds)]
	streamInput := stream.Input
	r.Inputs[0].Type = streamInput

	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, ds.Name),
			Enabled: true,
			DataStream: kibana.DataStream{
				Type:    ds.Type,
				Dataset: getDataStreamDataset(pkg, ds),
			},
		},
	}

	// Add dataStream-level vars
	streams[0].Vars = setKibanaVariables(stream.Vars, config.DataStream.Vars)
	r.Inputs[0].Streams = streams

	// Add input-level vars
	input := policyTemplate.FindInputByType(streamInput)
	if input != nil {
		r.Inputs[0].Vars = setKibanaVariables(input.Vars, config.Vars)
	}

	// Add package-level vars
	r.Vars = setKibanaVariables(pkg.Vars, config.Vars)

	return r
}

func createInputPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	config testConfig,
	suffix string,
) kibana.PackageDataStream {
	r := kibana.PackageDataStream{
		Name:      fmt.Sprintf("%s-%s-%s", pkg.Name, policyTemplate.Name, suffix),
		Namespace: kibanaPolicy.Namespace,
		PolicyID:  kibanaPolicy.ID,
		Enabled:   true,
	}
	r.Package.Name = pkg.Name
	r.Package.Title = pkg.Title
	r.Package.Version = pkg.Version
	r.Inputs = []kibana.Input{
		{
			PolicyTemplate: policyTemplate.Name,
			Enabled:        true,
			Vars:           kibana.Vars{},
		},
	}

	streamInput := policyTemplate.Input
	r.Inputs[0].Type = streamInput

	dataset := fmt.Sprintf("%s.%s", pkg.Name, policyTemplate.Name)
	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, policyTemplate.Name),
			Enabled: true,
			DataStream: kibana.DataStream{
				Type:    policyTemplate.Type,
				Dataset: dataset,
			},
		},
	}

	// Add policyTemplate-level vars.
	vars := setKibanaVariables(policyTemplate.Vars, config.Vars)
	if _, found := vars["data_stream.dataset"]; !found {
		dataStreamDataset := dataset
		v, _ := config.Vars.GetValue("data_stream.dataset")
		if dataset, ok := v.(string); ok && dataset != "" {
			dataStreamDataset = dataset
		}

		var value packages.VarValue
		value.Unpack(dataStreamDataset)
		vars["data_stream.dataset"] = kibana.Var{
			Value: value,
			Type:  "text",
		}
	}

	streams[0].Vars = vars
	r.Inputs[0].Streams = streams
	return r
}

func setKibanaVariables(definitions []packages.Variable, values common.MapStr) kibana.Vars {
	vars := kibana.Vars{}
	for _, definition := range definitions {
		val := definition.Default

		value, err := values.GetValue(definition.Name)
		if err == nil {
			val = packages.VarValue{}
			val.Unpack(value)
		}

		vars[definition.Name] = kibana.Var{
			Type:  definition.Type,
			Value: val,
		}
	}
	return vars
}

// getDataStreamIndex returns the index of the data stream whose input name
// matches. Otherwise it returns the 0.
func getDataStreamIndex(inputName string, ds packages.DataStreamManifest) int {
	for i, s := range ds.Streams {
		if s.Input == inputName {
			return i
		}
	}
	return 0
}

func getDataStreamDataset(pkg packages.PackageManifest, ds packages.DataStreamManifest) string {
	if len(ds.Dataset) > 0 {
		return ds.Dataset
	}
	return fmt.Sprintf("%s.%s", pkg.Name, ds.Name)
}

// findPolicyTemplateForInput returns the name of the policy_template that
// applies to the input under test. An error is returned if no policy template
// matches or if multiple policy templates match and the response is ambiguous.
func findPolicyTemplateForInput(pkg packages.PackageManifest, ds packages.DataStreamManifest, inputName string) (string, error) {
	if pkg.Type == "input" {
		return findPolicyTemplateForInputPackage(pkg, inputName)
	}
	return findPolicyTemplateForDataStream(pkg, ds, inputName)
}

func findPolicyTemplateForDataStream(pkg packages.PackageManifest, ds packages.DataStreamManifest, inputName string) (string, error) {
	if inputName == "" {
		if len(ds.Streams) == 0 {
			return "", errors.New("no streams declared in data stream manifest")
		}
		inputName = ds.Streams[getDataStreamIndex(inputName, ds)].Input
	}

	var matchedPolicyTemplates []string
	for _, policyTemplate := range pkg.PolicyTemplates {
		// Does this policy_template include this input type?
		if policyTemplate.FindInputByType(inputName) == nil {
			continue
		}

		// Does the policy_template apply to this data stream (when data streams are specified)?
		if len(policyTemplate.DataStreams) > 0 && !slices.Contains(policyTemplate.DataStreams, ds.Name) {
			continue
		}

		matchedPolicyTemplates = append(matchedPolicyTemplates, policyTemplate.Name)
	}

	switch len(matchedPolicyTemplates) {
	case 1:
		return matchedPolicyTemplates[0], nil
	case 0:
		return "", fmt.Errorf("no policy template was found for data stream %q "+
			"with input type %q: verify that you have included the data stream "+
			"and input in the package's policy_template list", ds.Name, inputName)
	default:
		return "", fmt.Errorf("ambiguous result: multiple policy templates ([%s]) "+
			"were found that apply to data stream %q with input type %q: please "+
			"specify the 'policy_template' in the system test config",
			strings.Join(matchedPolicyTemplates, ", "), ds.Name, inputName)
	}
}

func findPolicyTemplateForInputPackage(pkg packages.PackageManifest, inputName string) (string, error) {
	if inputName == "" {
		if len(pkg.PolicyTemplates) == 0 {
			return "", errors.New("no policy templates specified for input package")
		}
		inputName = pkg.PolicyTemplates[0].Input
	}

	var matched []string
	for _, policyTemplate := range pkg.PolicyTemplates {
		if policyTemplate.Input != inputName {
			continue
		}

		matched = append(matched, policyTemplate.Name)
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", fmt.Errorf("no policy template was found"+
			"with input type %q: verify that you have included the data stream "+
			"and input in the package's policy_template list", inputName)
	default:
		return "", fmt.Errorf("ambiguous result: multiple policy templates ([%s]) "+
			"with input type %q: please "+
			"specify the 'policy_template' in the system test config",
			strings.Join(matched, ", "), inputName)
	}
}

func selectPolicyTemplateByName(policies []packages.PolicyTemplate, name string) (packages.PolicyTemplate, error) {
	for _, policy := range policies {
		if policy.Name == name {
			return policy, nil
		}
	}
	return packages.PolicyTemplate{}, fmt.Errorf("policy template %q not found", name)
}

func (r *runner) checkTransforms(ctx context.Context, config *testConfig, pkgManifest *packages.PackageManifest, ds kibana.PackageDataStream, dataStream string) error {
	transforms, err := packages.ReadTransformsFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return fmt.Errorf("loading transforms for package failed (root: %s): %w", r.options.PackageRootPath, err)
	}
	for _, transform := range transforms {
		hasSource, err := transform.HasSource(dataStream)
		if err != nil {
			return fmt.Errorf("failed to check if transform %q has %s as source: %w", transform.Name, dataStream, err)
		}
		if !hasSource {
			logger.Debugf("transform %q does not match %q as source (sources: %s)", transform.Name, dataStream, transform.Definition.Source.Index)
			continue
		}

		logger.Debugf("checking transform %q", transform.Name)

		// IDs format is: "<type>-<package>.<transform>-<namespace>-<version>"
		// For instance: "logs-ti_anomali.latest_ioc-default-0.1.0"
		transformPattern := fmt.Sprintf("%s-%s.%s-*-%s",
			ds.Inputs[0].Streams[0].DataStream.Type,
			pkgManifest.Name,
			transform.Name,
			transform.Definition.Meta.FleetTransformVersion,
		)
		transformId, err := r.getTransformId(ctx, transformPattern)
		if err != nil {
			return fmt.Errorf("failed to determine transform ID: %w", err)
		}

		// Using the preview instead of checking the actual index because
		// transforms with retention policies may be deleting the documents based
		// on old fixtures as soon as they are indexed.
		transformDocs, err := r.previewTransform(ctx, transformId)
		if err != nil {
			return fmt.Errorf("failed to preview transform %q: %w", transformId, err)
		}
		if len(transformDocs) == 0 {
			return fmt.Errorf("no documents found in preview for transform %q", transformId)
		}

		transformRootPath := filepath.Dir(transform.Path)
		fieldsValidator, err := fields.CreateValidatorForDirectory(transformRootPath,
			fields.WithSpecVersion(pkgManifest.SpecVersion),
			fields.WithNumericKeywordFields(config.NumericKeywordFields),
			fields.WithEnabledImportAllECSSChema(true),
		)
		if err != nil {
			return fmt.Errorf("creating fields validator for data stream failed (path: %s): %w", transformRootPath, err)
		}
		if err := validateFields(transformDocs, fieldsValidator, dataStream); err != nil {
			return err
		}
	}

	return nil
}

func (r *runner) getTransformId(ctx context.Context, transformPattern string) (string, error) {
	resp, err := r.options.API.TransformGetTransform(
		r.options.API.TransformGetTransform.WithContext(ctx),
		r.options.API.TransformGetTransform.WithTransformID(transformPattern),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return "", fmt.Errorf("failed to get transforms: %s", resp.String())
	}

	var transforms struct {
		Transforms []struct {
			ID string `json:"id"`
		} `json:"transforms"`
	}

	err = json.NewDecoder(resp.Body).Decode(&transforms)
	switch {
	case err != nil:
		return "", fmt.Errorf("failed to decode response: %w", err)
	case len(transforms.Transforms) == 0:
		return "", fmt.Errorf("no transform found with pattern %q", transformPattern)
	case len(transforms.Transforms) > 1:
		return "", fmt.Errorf("multiple transforms (%d) found with pattern %q", len(transforms.Transforms), transformPattern)
	}
	id := transforms.Transforms[0].ID
	if id == "" {
		return "", fmt.Errorf("empty ID found with pattern %q", transformPattern)
	}
	return id, nil
}

func (r *runner) previewTransform(ctx context.Context, transformId string) ([]common.MapStr, error) {
	resp, err := r.options.API.TransformPreviewTransform(
		r.options.API.TransformPreviewTransform.WithContext(ctx),
		r.options.API.TransformPreviewTransform.WithTransformID(transformId),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("failed to preview transform %q: %s", transformId, resp.String())
	}

	var preview struct {
		Documents []common.MapStr `json:"preview"`
	}
	err = json.NewDecoder(resp.Body).Decode(&preview)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return preview.Documents, nil
}

func deleteDataStreamDocs(ctx context.Context, api *elasticsearch.API, dataStream string) error {
	body := strings.NewReader(`{ "query": { "match_all": {} } }`)
	resp, err := api.DeleteByQuery([]string{dataStream}, body,
		api.DeleteByQuery.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to delete data stream docs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Unavailable index is ok, this means that data is already not there.
		logger.Debugf("Failed but ignored with status not found %s: %s", dataStream, resp.String())
		return nil
	}
	if resp.IsError() {
		return fmt.Errorf("failed to delete data stream docs for data stream %s: %s", dataStream, resp.String())
	}

	return nil
}

func filterAgents(allAgents []kibana.Agent, svcInfo servicedeployer.ServiceInfo) []kibana.Agent {
	if svcInfo.Agent.Host.NamePrefix != "" {
		logger.Debugf("filter agents using criteria: NamePrefix=%s", svcInfo.Agent.Host.NamePrefix)
	}

	var filtered []kibana.Agent
	for _, agent := range allAgents {
		if agent.PolicyRevision == 0 {
			continue // For some reason Kibana doesn't always return a valid policy revision (eventually it will be present and valid)
		}

		if svcInfo.Agent.Host.NamePrefix != "" && !strings.HasPrefix(agent.LocalMetadata.Host.Name, svcInfo.Agent.Host.NamePrefix) {
			continue
		}
		filtered = append(filtered, agent)
	}
	return filtered
}

func filterIndependentAgents(allAgents []kibana.Agent, agentInfo agentdeployer.AgentInfo) []kibana.Agent {
	// filtered list of agents must contain all agents started by the stack
	// they could be assigned the default policy (elastic-agent-managed-ep) or the test policy (ep-test-system-*)
	var filtered []kibana.Agent
	for _, agent := range allAgents {
		if agent.PolicyRevision == 0 {
			continue // For some reason Kibana doesn't always return a valid policy revision (eventually it will be present and valid)
		}

		if agent.Status != "online" {
			continue
		}

		if agent.PolicyID != agentInfo.Policy.ID {
			continue
		}

		filtered = append(filtered, agent)
	}
	return filtered
}

func writeSampleEvent(path string, doc common.MapStr, specVersion semver.Version) error {
	jsonFormatter := formatter.JSONFormatterBuilder(specVersion)
	body, err := jsonFormatter.Encode(doc)
	if err != nil {
		return fmt.Errorf("marshalling sample event failed: %w", err)
	}

	err = os.WriteFile(filepath.Join(path, "sample_event.json"), body, 0644)
	if err != nil {
		return fmt.Errorf("writing sample event failed: %w", err)
	}

	return nil
}

func validateFields(docs []common.MapStr, fieldsValidator *fields.Validator, dataStream string) error {
	var multiErr multierror.Error
	for _, doc := range docs {
		if message, err := doc.GetValue("error.message"); err != common.ErrKeyNotFound {
			multiErr = append(multiErr, fmt.Errorf("found error.message in event: %v", message))
			continue
		}

		errs := fieldsValidator.ValidateDocumentMap(doc)
		if errs != nil {
			multiErr = append(multiErr, errs...)
			continue
		}
	}

	if len(multiErr) > 0 {
		multiErr = multiErr.Unique()
		return testrunner.ErrTestCaseFailed{
			Reason:  fmt.Sprintf("one or more errors found in documents stored in %s data stream", dataStream),
			Details: multiErr.Error(),
		}
	}

	return nil
}

func validateIgnoredFields(stackVersionString string, scenario *scenarioTest, config *testConfig) error {
	skipIgnoredFields := append([]string(nil), config.SkipIgnoredFields...)
	stackVersion, err := semver.NewVersion(stackVersionString)
	if err != nil {
		return fmt.Errorf("failed to parse stack version: %w", err)
	}
	if stackVersion.LessThan(semver.MustParse("8.14.0")) {
		// Pre 8.14 Elasticsearch commonly has event.original not mapped correctly, exclude from check: https://github.com/elastic/elasticsearch/pull/106714
		skipIgnoredFields = append(skipIgnoredFields, "event.original")
	}

	ignoredFields := make([]string, 0, len(scenario.ignoredFields))

	for _, field := range scenario.ignoredFields {
		if !slices.Contains(skipIgnoredFields, field) {
			ignoredFields = append(ignoredFields, field)
		}
	}

	if len(ignoredFields) > 0 {
		issues := make([]struct {
			ID            any `json:"_id"`
			Timestamp     any `json:"@timestamp,omitempty"`
			IgnoredFields any `json:"ignored_field_values"`
		}, len(scenario.degradedDocs))
		for i, d := range scenario.degradedDocs {
			issues[i].ID = d["_id"]
			if source, ok := d["_source"].(map[string]any); ok {
				if ts, ok := source["@timestamp"]; ok {
					issues[i].Timestamp = ts
				}
			}
			issues[i].IgnoredFields = d["ignored_field_values"]
		}
		degradedDocsJSON, err := json.MarshalIndent(issues, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal degraded docs to JSON: %w", err)
		}

		return fmt.Errorf("found ignored fields in data stream %s: %v. Affected documents: %s", scenario.dataStream, ignoredFields, degradedDocsJSON)
	}

	return nil
}

func assertHitCount(expected int, docs []common.MapStr) (pass bool, message string) {
	if expected != 0 {
		observed := len(docs)
		logger.Debugf("assert hit count expected %d, observed %d", expected, observed)
		if observed != expected {
			return false, fmt.Sprintf("observed hit count %d did not match expected hit count %d", observed, expected)
		}
	}
	return true, ""
}

func (r *runner) selectVariants(variantsFile *servicedeployer.VariantsFile) []string {
	if variantsFile == nil || variantsFile.Variants == nil {
		return []string{""} // empty variants file switches to no-variant mode
	}

	var variantNames []string
	for k := range variantsFile.Variants {
		if r.options.ServiceVariant != "" && r.options.ServiceVariant != k {
			continue
		}
		variantNames = append(variantNames, k)
	}
	return variantNames
}

func (r *runner) generateTestResult(docs []common.MapStr, specVersion semver.Version) error {
	if !r.options.GenerateTestResult {
		return nil
	}

	rootPath := r.options.PackageRootPath
	if ds := r.options.TestFolder.DataStream; ds != "" {
		rootPath = filepath.Join(rootPath, "data_stream", ds)
	}

	if err := writeSampleEvent(rootPath, docs[0], specVersion); err != nil {
		return fmt.Errorf("failed to write sample event file: %w", err)
	}

	return nil
}

func (r *runner) checkNewAgentLogs(ctx context.Context, agent agentdeployer.DeployedAgent, startTesting time.Time, errorPatterns []logsByContainer) (results []testrunner.TestResult, err error) {
	if agent == nil {
		return nil, nil
	}

	f, err := os.CreateTemp("", "elastic-agent.logs")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for logs: %w", err)
	}
	defer os.Remove(f.Name())

	for _, patternsContainer := range errorPatterns {
		if patternsContainer.containerName != "elastic-agent" {
			continue
		}

		startTime := time.Now()

		outputBytes, err := agent.Logs(ctx, startTesting)
		if err != nil {
			return nil, fmt.Errorf("check log messages failed: %s", err)
		}
		_, err = f.Write(outputBytes)
		if err != nil {
			return nil, fmt.Errorf("write log messages failed: %s", err)
		}

		err = r.anyErrorMessages(f.Name(), startTesting, patternsContainer.patterns)
		if e, ok := err.(testrunner.ErrTestCaseFailed); ok {
			tr := testrunner.TestResult{
				TestType:   TestType,
				Name:       fmt.Sprintf("(%s logs)", patternsContainer.containerName),
				Package:    r.options.TestFolder.Package,
				DataStream: r.options.TestFolder.DataStream,
			}
			tr.FailureMsg = e.Error()
			tr.FailureDetails = e.Details
			tr.TimeElapsed = time.Since(startTime)
			results = append(results, tr)
			// Just check elastic-agent
			break
		}

		if err != nil {
			return nil, fmt.Errorf("check log messages failed: %s", err)
		}
		// Just check elastic-agent
		break
	}
	return results, nil
}

func (r *runner) checkAgentLogs(dump []stack.DumpResult, startTesting time.Time, errorPatterns []logsByContainer) (results []testrunner.TestResult, err error) {
	for _, patternsContainer := range errorPatterns {
		startTime := time.Now()

		serviceDumpIndex := slices.IndexFunc(dump, func(d stack.DumpResult) bool {
			return d.ServiceName == patternsContainer.containerName
		})
		if serviceDumpIndex < 0 {
			return nil, fmt.Errorf("could not find logs dump for service %s", patternsContainer.containerName)
		}
		serviceLogsFile := dump[serviceDumpIndex].LogsFile

		err = r.anyErrorMessages(serviceLogsFile, startTesting, patternsContainer.patterns)
		if e, ok := err.(testrunner.ErrTestCaseFailed); ok {
			tr := testrunner.TestResult{
				TestType:   TestType,
				Name:       fmt.Sprintf("(%s logs)", patternsContainer.containerName),
				Package:    r.options.TestFolder.Package,
				DataStream: r.options.TestFolder.DataStream,
			}
			tr.FailureMsg = e.Error()
			tr.FailureDetails = e.Details
			tr.TimeElapsed = time.Since(startTime)
			results = append(results, tr)
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("check log messages failed: %s", err)
		}
	}
	return results, nil
}

func (r *runner) anyErrorMessages(logsFilePath string, startTime time.Time, errorPatterns []logsRegexp) error {
	var multiErr multierror.Error
	processLog := func(log stack.LogLine) error {
		for _, pattern := range errorPatterns {
			if !pattern.includes.MatchString(log.Message) {
				continue
			}
			isExcluded := false
			for _, excludes := range pattern.excludes {
				if excludes.MatchString(log.Message) {
					isExcluded = true
					break
				}
			}
			if isExcluded {
				continue
			}

			multiErr = append(multiErr, fmt.Errorf("found error %q", log.Message))
		}
		return nil
	}
	err := stack.ParseLogs(stack.ParseLogsOptions{
		LogsFilePath: logsFilePath,
		StartTime:    startTime,
	}, processLog)
	if err != nil {
		return err
	}

	if len(multiErr) > 0 {
		return testrunner.ErrTestCaseFailed{
			Reason:  fmt.Sprintf("one or more errors found while examining %s", filepath.Base(logsFilePath)),
			Details: multiErr.Error(),
		}
	}
	return nil
}
