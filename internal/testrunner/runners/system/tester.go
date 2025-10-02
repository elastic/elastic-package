// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/wait"
)

const (
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

	// TestType defining system tests
	TestType testrunner.TestType = "system"

	// Maximum number of events to query.
	elasticsearchQuerySize = 500

	// ServiceLogsAgentDir is folder path where log files produced by the service
	// are stored on the Agent container's filesystem.
	ServiceLogsAgentDir = "/tmp/service_logs"

	waitForDataDefaultTimeout = 10 * time.Minute

	otelCollectorInputName = "otelcol"
	otelSuffixDataset      = "otel"
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
				{
					// HTTPJSON template error.
					includes: regexp.MustCompile(`^error processing response: template: :\d+:\d+: executing "" at <`),
					excludes: []*regexp.Regexp{
						// Unfortunate: https://github.com/elastic/beats/issues/34544
						// See also https://github.com/elastic/beats/pull/39929.
						regexp.MustCompile(`: map has no entry for key`),
						regexp.MustCompile(`: can't evaluate field (?:[^ ]+) in type interface`),
					},
				},
			},
		},
	}
	enableIndependentAgentsEnv   = environment.WithElasticPackagePrefix("TEST_ENABLE_INDEPENDENT_AGENT")
	dumpScenarioDocsEnv          = environment.WithElasticPackagePrefix("TEST_DUMP_SCENARIO_DOCS")
	fieldValidationTestMethodEnv = environment.WithElasticPackagePrefix("FIELD_VALIDATION_TEST_METHOD")
	prefixServiceTestRunIDEnv    = environment.WithElasticPackagePrefix("PREFIX_SERVICE_TEST_RUN_ID")
)

type fieldValidationMethod int

const (
	// Required to allow setting `fields` as an option via environment variable
	fieldsMethod fieldValidationMethod = iota
	mappingsMethod
)

var validationMethods = map[string]fieldValidationMethod{
	"fields":   fieldsMethod,
	"mappings": mappingsMethod,
}

type tester struct {
	profile            *profile.Profile
	testFolder         testrunner.TestFolder
	packageRootPath    string
	generateTestResult bool
	esAPI              *elasticsearch.API
	esClient           *elasticsearch.Client
	kibanaClient       *kibana.Client

	runIndependentElasticAgent bool

	fieldValidationMethod fieldValidationMethod

	deferCleanup   time.Duration
	serviceVariant string
	configFileName string

	runSetup     bool
	runTearDown  bool
	runTestsOnly bool

	pipelines []ingest.Pipeline

	dataStreamPath     string
	stackVersion       kibana.VersionInfo
	locationManager    *locations.LocationManager
	resourcesManager   *resources.Manager
	pkgManifest        *packages.PackageManifest
	dataStreamManifest *packages.DataStreamManifest
	withCoverage       bool
	coverageType       string

	serviceStateFilePath string

	globalTestConfig testrunner.GlobalRunnerTestConfig

	// Execution order of following handlers is defined in runner.TearDown() method.
	removeAgentHandler        func(context.Context) error
	deleteTestPolicyHandler   func(context.Context) error
	cleanTestScenarioHandler  func(context.Context) error
	resetAgentPolicyHandler   func(context.Context) error
	resetAgentLogLevelHandler func(context.Context) error
	shutdownServiceHandler    func(context.Context) error
	shutdownAgentHandler      func(context.Context) error
}

type SystemTesterOptions struct {
	Profile            *profile.Profile
	TestFolder         testrunner.TestFolder
	PackageRootPath    string
	GenerateTestResult bool
	API                *elasticsearch.API
	KibanaClient       *kibana.Client

	// FIXME: Keeping Elasticsearch client to be able to do low-level requests for parameters not supported yet by the API.
	ESClient *elasticsearch.Client

	DeferCleanup     time.Duration
	ServiceVariant   string
	ConfigFileName   string
	GlobalTestConfig testrunner.GlobalRunnerTestConfig
	WithCoverage     bool
	CoverageType     string

	RunSetup     bool
	RunTearDown  bool
	RunTestsOnly bool
}

func NewSystemTester(options SystemTesterOptions) (*tester, error) {
	r := tester{
		profile:                    options.Profile,
		testFolder:                 options.TestFolder,
		packageRootPath:            options.PackageRootPath,
		generateTestResult:         options.GenerateTestResult,
		esAPI:                      options.API,
		esClient:                   options.ESClient,
		kibanaClient:               options.KibanaClient,
		deferCleanup:               options.DeferCleanup,
		serviceVariant:             options.ServiceVariant,
		configFileName:             options.ConfigFileName,
		runSetup:                   options.RunSetup,
		runTestsOnly:               options.RunTestsOnly,
		runTearDown:                options.RunTearDown,
		globalTestConfig:           options.GlobalTestConfig,
		withCoverage:               options.WithCoverage,
		coverageType:               options.CoverageType,
		runIndependentElasticAgent: true,
	}
	r.resourcesManager = resources.NewManager()
	r.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: r.kibanaClient})

	r.serviceStateFilePath = filepath.Join(stateFolderPath(r.profile.ProfilePath), serviceStateFileName)

	var err error

	r.locationManager, err = locations.NewLocationManager()
	if err != nil {
		return nil, fmt.Errorf("reading service logs directory failed: %w", err)
	}

	r.dataStreamPath, _, err = packages.FindDataStreamRootForPath(r.testFolder.Path)
	if err != nil {
		return nil, fmt.Errorf("locating data stream root failed: %w", err)
	}

	if r.esAPI == nil {
		return nil, errors.New("missing Elasticsearch client")
	}
	if r.kibanaClient == nil {
		return nil, errors.New("missing Kibana client")
	}

	r.stackVersion, err = r.kibanaClient.Version()
	if err != nil {
		return nil, fmt.Errorf("cannot request Kibana version: %w", err)
	}

	r.pkgManifest, err = packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed: %w", err)
	}

	if r.dataStreamPath != "" {
		// Avoid reading data stream manifest if path is empty (e.g. input packages) to avoid
		// filling "r.dataStreamManifest" with values from package manifest since the resulting path will point to
		// the package manifest instead of the data stream manifest.
		r.dataStreamManifest, err = packages.ReadDataStreamManifest(filepath.Join(r.dataStreamPath, packages.DataStreamManifestFile))
		if err != nil {
			return nil, fmt.Errorf("reading data stream manifest failed: %w", err)
		}
	}

	// If the environment variable is present, it always has preference over the root
	// privileges value (if any) defined in the manifest file
	v, ok := os.LookupEnv(enableIndependentAgentsEnv)
	if ok {
		r.runIndependentElasticAgent = strings.ToLower(v) == "true"
	}

	// default method to validate using mappings (along with fields)
	r.fieldValidationMethod = mappingsMethod
	v, ok = os.LookupEnv(fieldValidationTestMethodEnv)
	if ok {
		method, ok := validationMethods[v]
		if !ok {
			return nil, fmt.Errorf("invalid field method option: %s", v)
		}
		r.fieldValidationMethod = method
	}

	return &r, nil
}

// Ensures that runner implements testrunner.Tester interface
var _ testrunner.Tester = new(tester)

// Type returns the type of test that can be run by this test runner.
func (r *tester) Type() testrunner.TestType {
	return TestType
}

// String returns the human-friendly name of the test runner.
func (r *tester) String() string {
	return "system"
}

// Parallel indicates if this tester can run in parallel or not.
func (r tester) Parallel() bool {
	// it is required independent Elastic Agents to run in parallel system tests
	return r.runIndependentElasticAgent && r.globalTestConfig.Parallel
}

// Run runs the system tests defined under the given folder
func (r *tester) Run(ctx context.Context) ([]testrunner.TestResult, error) {
	stackConfig, err := stack.LoadConfig(r.profile)
	if err != nil {
		return nil, err
	}

	if !r.runSetup && !r.runTearDown && !r.runTestsOnly {
		return r.run(ctx, stackConfig)
	}

	result := r.newResult("(init)")

	svcInfo, err := r.createServiceInfo()
	if err != nil {
		return result.WithError(err)
	}

	configFile := filepath.Join(r.testFolder.Path, r.configFileName)
	testConfig, err := newConfig(configFile, svcInfo, r.serviceVariant)
	if err != nil {
		return nil, fmt.Errorf("unable to load system test case file '%s': %w", configFile, err)
	}
	logger.Debugf("Using config: %q", testConfig.Name())

	resultName := ""
	switch {
	case r.runSetup:
		resultName = "setup"
	case r.runTearDown:
		resultName = "teardown"
	case r.runTestsOnly:
		resultName = "tests"
	}
	result = r.newResult(fmt.Sprintf("%s - %s", resultName, testConfig.Name()))

	scenario, err := r.prepareScenario(ctx, testConfig, stackConfig, svcInfo)
	if r.runSetup && err != nil {
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

	if r.runTestsOnly {
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

	if r.runTearDown {
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

func (r *tester) createAgentOptions(policyName, deployerName string) agentdeployer.FactoryOptions {
	return agentdeployer.FactoryOptions{
		Profile:            r.profile,
		PackageRootPath:    r.packageRootPath,
		DataStreamRootPath: r.dataStreamPath,
		DevDeployDir:       DevDeployDir,
		Type:               agentdeployer.TypeTest,
		StackVersion:       r.stackVersion.Version(),
		PackageName:        r.testFolder.Package,
		DataStream:         r.testFolder.DataStream,
		PolicyName:         policyName,
		DeployerName:       deployerName,
		RunTearDown:        r.runTearDown,
		RunTestsOnly:       r.runTestsOnly,
		RunSetup:           r.runSetup,
	}
}

func (r *tester) createServiceOptions(variantName, deployerName string) servicedeployer.FactoryOptions {
	return servicedeployer.FactoryOptions{
		Profile:                r.profile,
		PackageRootPath:        r.packageRootPath,
		DataStreamRootPath:     r.dataStreamPath,
		DevDeployDir:           DevDeployDir,
		Variant:                variantName,
		Type:                   servicedeployer.TypeTest,
		StackVersion:           r.stackVersion.Version(),
		RunTearDown:            r.runTearDown,
		RunTestsOnly:           r.runTestsOnly,
		RunSetup:               r.runSetup,
		DeployIndependentAgent: r.runIndependentElasticAgent,
		DeployerName:           deployerName,
	}
}

func (r *tester) createAgentInfo(policy *kibana.Policy, config *testConfig, runID string) (agentdeployer.AgentInfo, error) {
	var info agentdeployer.AgentInfo

	info.Name = r.testFolder.Package
	info.Logs.Folder.Agent = ServiceLogsAgentDir
	info.Test.RunID = runID

	dirPath, err := agentdeployer.CreateServiceLogsDir(r.profile, r.packageRootPath, r.testFolder.DataStream, runID)
	if err != nil {
		return agentdeployer.AgentInfo{}, fmt.Errorf("failed to create service logs dir: %w", err)
	}
	info.Logs.Folder.Local = dirPath

	info.Policy.Name = policy.Name
	info.Policy.ID = policy.ID

	// Copy all agent settings from the test configuration file
	info.Agent.AgentSettings = config.Agent.AgentSettings

	// If user is defined in the configuration file, it has preference
	// and it should not be overwritten by the value in the package or DataStream manifest
	if info.Agent.User == "" && r.agentRequiresRootPrivileges() {
		info.Agent.User = "root"
	}

	if info.Agent.User == "root" {
		// Ensure that CAP_CHOWN is present if the user for testing is root
		if !slices.Contains(info.Agent.LinuxCapabilities, "CAP_CHOWN") {
			info.Agent.LinuxCapabilities = append(info.Agent.LinuxCapabilities, "CAP_CHOWN")
		}
	}

	// This could be removed once package-spec adds this new field
	if !slices.Contains([]string{"", "default", "complete", "systemd"}, info.Agent.BaseImage) {
		return agentdeployer.AgentInfo{}, fmt.Errorf("invalid value for agent.base_image: %q", info.Agent.BaseImage)
	}

	return info, nil
}

func (r *tester) agentRequiresRootPrivileges() bool {
	if r.pkgManifest.Agent.Privileges.Root {
		return true
	}
	if r.dataStreamManifest != nil && r.dataStreamManifest.Agent.Privileges.Root {
		return true
	}
	return false
}

func (r *tester) createServiceInfo() (servicedeployer.ServiceInfo, error) {
	var svcInfo servicedeployer.ServiceInfo
	svcInfo.Name = r.testFolder.Package
	svcInfo.Logs.Folder.Local = r.locationManager.ServiceLogDir()
	svcInfo.Logs.Folder.Agent = ServiceLogsAgentDir

	prefix := ""
	if v, found := os.LookupEnv(prefixServiceTestRunIDEnv); found && v != "" {
		prefix = v
	}
	svcInfo.Test.RunID = common.CreateTestRunIDWithPrefix(prefix)

	if r.runTearDown || r.runTestsOnly {
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
func (r *tester) TearDown(ctx context.Context) error {
	return nil
}

func (r *tester) tearDownTest(ctx context.Context) error {
	if r.deferCleanup > 0 {
		logger.Debugf("waiting for %s before tearing down...", r.deferCleanup)
		select {
		case <-time.After(r.deferCleanup):
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

	if r.shutdownAgentHandler != nil {
		if err := r.shutdownAgentHandler(cleanupCtx); err != nil {
			return err
		}
		r.shutdownAgentHandler = nil
	}

	if r.deleteTestPolicyHandler != nil {
		if err := r.deleteTestPolicyHandler(cleanupCtx); err != nil {
			return err
		}
		r.deleteTestPolicyHandler = nil
	}

	return nil
}

func (r *tester) newResult(name string) *testrunner.ResultComposer {
	return testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   TestType,
		Name:       name,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	})
}

func (r *tester) run(ctx context.Context, stackConfig stack.Config) (results []testrunner.TestResult, err error) {
	result := r.newResult("(init)")

	startTesting := time.Now()

	results, err = r.runTestPerVariant(ctx, stackConfig, result, r.configFileName, r.serviceVariant)
	if err != nil {
		return results, err
	}

	// Every tester is in charge of just one test, so if there is no error,
	// then there should be just one result for tests. As an exception, there could
	// be two results if there is any issue checking Elastic Agent logs.
	if len(results) > 0 && results[0].Skipped != nil {
		logger.Debugf("Test skipped, avoid checking agent logs")
		return results, nil
	}

	tempDir, err := os.MkdirTemp("", "test-system-")
	if err != nil {
		return nil, fmt.Errorf("can't create temporal directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	provider, err := stack.BuildProvider(stackConfig.Provider, r.profile)
	if err != nil {
		return nil, fmt.Errorf("failed to build stack provider: %w", err)
	}

	dumpOptions := stack.DumpOptions{
		Output:  tempDir,
		Profile: r.profile,
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

func (r *tester) runTestPerVariant(ctx context.Context, stackConfig stack.Config, result *testrunner.ResultComposer, cfgFile, variantName string) ([]testrunner.TestResult, error) {
	svcInfo, err := r.createServiceInfo()
	if err != nil {
		return result.WithError(err)
	}

	configFile := filepath.Join(r.testFolder.Path, cfgFile)
	testConfig, err := newConfig(configFile, svcInfo, variantName)
	if err != nil {
		return nil, fmt.Errorf("unable to load system test case file '%s': %w", configFile, err)
	}
	logger.Debugf("Using config: %q", testConfig.Name())

	partial, err := r.runTest(ctx, testConfig, stackConfig, svcInfo)

	tdErr := r.tearDownTest(ctx)
	if err != nil {
		return partial, err
	}
	if tdErr != nil {
		return partial, fmt.Errorf("failed to tear down runner: %w", tdErr)
	}
	return partial, nil
}

func isSyntheticSourceModeEnabled(ctx context.Context, api *elasticsearch.API, dataStreamName string) (bool, error) {
	// We append a suffix so we don't use an existing resource, what may cause conflicts in old versions of
	// Elasticsearch, such as https://github.com/elastic/elasticsearch/issues/84256.
	resp, err := api.Indices.SimulateIndexTemplate(dataStreamName+"simulated",
		api.Indices.SimulateIndexTemplate.WithContext(ctx),
	)
	if err != nil {
		return false, fmt.Errorf("could not simulate index template for %s: %w", dataStreamName, err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return false, fmt.Errorf("could not simulate index template for %s: %s", dataStreamName, resp.String())
	}

	var results struct {
		Template struct {
			Mappings struct {
				Source struct {
					Mode string `json:"mode"`
				} `json:"_source"`
			} `json:"mappings"`
			Settings struct {
				Index struct {
					Mode    string `json:"mode"`
					Mapping struct {
						Source struct {
							Mode string `json:"mode"`
						} `json:"source"`
					} `json:"mapping"`
				} `json:"index"`
			} `json:"settings"`
		} `json:"template"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return false, fmt.Errorf("could not decode index template simulation response: %w", err)
	}

	// in 8.17.2 source mode definition is now under settings object
	if results.Template.Mappings.Source.Mode == "synthetic" || results.Template.Settings.Index.Mapping.Source.Mode == "synthetic" {
		return true, nil
	}

	// It seems that some index modes enable synthetic source mode even when it is not explicitly mentioned
	// in the mappings. So assume that when these index modes are used, the synthetic mode is also used.
	syntheticsIndexModes := []string{
		"logs", // Replaced in 8.15.0 with "logsdb", see https://github.com/elastic/elasticsearch/pull/111054
		"logsdb",
		"time_series",
	}
	if slices.Contains(syntheticsIndexModes, results.Template.Settings.Index.Mode) {
		return true, nil
	}

	return false, nil
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

func (r *tester) getDocs(ctx context.Context, dataStream string) (*hits, error) {
	resp, err := r.esAPI.Search(
		r.esAPI.Search.WithContext(ctx),
		r.esAPI.Search.WithIndex(dataStream),
		r.esAPI.Search.WithSort("@timestamp:asc"),
		r.esAPI.Search.WithSize(elasticsearchQuerySize),
		r.esAPI.Search.WithSource("true"),
		r.esAPI.Search.WithBody(strings.NewReader(checkFieldsBody)),
		r.esAPI.Search.WithIgnoreUnavailable(true),
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

type deprecationWarning struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	URL     string `json:"url"`
	Details string `json:"details"`

	ResolveDuringRollingUpgrade bool `json:"resolve_during_rolling_upgrade"`

	index string
}

func (r *tester) getDeprecationWarnings(ctx context.Context, dataStream string) ([]deprecationWarning, error) {
	config, err := stack.LoadConfig(r.profile)
	if err != nil {
		return []deprecationWarning{}, fmt.Errorf("failed to load config from profile: %w", err)
	}
	if config.Provider == stack.ProviderServerless {
		logger.Tracef("Skip deprecation warnings validation in Serverless projects")
		// In serverless, there is no handler for this request. Ignore this validation.
		// Example of response: [400 Bad Request] {"error":"no handler found for uri [/metrics-elastic_package_registry.metrics-62481/_migration/deprecations] and method [GET]"}
		return []deprecationWarning{}, nil
	}
	resp, err := r.esAPI.Migration.Deprecations(
		r.esAPI.Migration.Deprecations.WithContext(ctx),
		r.esAPI.Migration.Deprecations.WithIndex(dataStream),
	)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		if config.Provider == stack.ProviderEnvironment {
			// Ignore errors in provider environment too, in this case it could also be a Serverless project.
			logger.Tracef("Ignored deprecation warnings bad request code error in provider environment, response: %s", resp.String())
			return []deprecationWarning{}, nil
		}
	}

	if resp.IsError() {
		return nil, fmt.Errorf("unexpected status code in response: %s", resp.String())
	}

	// Apart from index_settings, there are also cluster_settings, node_settings and ml_settings.
	// There is also a data_streams field in the response that is not documented and is empty.
	// Here we are interested only on warnings on index settings.
	var results struct {
		IndexSettings map[string][]deprecationWarning `json:"index_settings"`
	}
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		return nil, fmt.Errorf("cannot decode response: %w", err)
	}

	var result []deprecationWarning
	for index, warnings := range results.IndexSettings {
		for _, warning := range warnings {
			warning.index = index
			result = append(result, warning)
		}
	}
	return result, nil
}

func (r *tester) checkDeprecationWarnings(stackVersion *semver.Version, warnings []deprecationWarning, configName string) []testrunner.TestResult {
	var results []testrunner.TestResult
	for _, warning := range warnings {
		if ignoredDeprecationWarning(stackVersion, warning) {
			continue
		}
		details := warning.Details
		if warning.index != "" {
			details = fmt.Sprintf("%s (index: %s)", details, warning.index)
		}
		tr := testrunner.TestResult{
			TestType:       TestType,
			Name:           "Deprecation warnings - " + configName,
			Package:        r.testFolder.Package,
			DataStream:     r.testFolder.DataStream,
			FailureMsg:     warning.Message,
			FailureDetails: details,
		}
		results = append(results, tr)
	}
	return results
}

func mustParseConstraint(c string) *semver.Constraints {
	constraint, err := semver.NewConstraint(c)
	if err != nil {
		panic(err)
	}
	return constraint
}

var ignoredWarnings = []struct {
	constraints *semver.Constraints
	pattern     *regexp.Regexp
}{
	{
		// This deprecation warning was introduced in 8.17.0 and fixed in Fleet in 8.17.2.
		// See https://github.com/elastic/kibana/pull/207133
		// Ignoring it because packages cannot do much about this on these versions.
		constraints: mustParseConstraint(`>=8.17.0,<8.17.2`),
		pattern:     regexp.MustCompile(`^Configuring source mode in mappings is deprecated and will be removed in future versions.`),
	},
}

func ignoredDeprecationWarning(stackVersion *semver.Version, warning deprecationWarning) bool {
	for _, rule := range ignoredWarnings {
		if rule.constraints != nil && !rule.constraints.Check(stackVersion) {
			continue
		}
		if rule.pattern.MatchString(warning.Message) {
			return true
		}
	}
	return false
}

type scenarioTest struct {
	// dataStream is the name of the target data stream where documents are indexed
	dataStream          string
	indexTemplateName   string
	policyTemplateName  string
	policyTemplateInput string
	kibanaDataStream    kibana.PackageDataStream
	syntheticEnabled    bool
	docs                []common.MapStr
	deprecationWarnings []deprecationWarning
	ignoredFields       []string
	degradedDocs        []common.MapStr
	agent               agentdeployer.DeployedAgent
	startTestTime       time.Time
}

func (r *tester) deleteDataStream(ctx context.Context, dataStream string) error {
	resp, err := r.esAPI.Indices.DeleteDataStream([]string{dataStream},
		r.esAPI.Indices.DeleteDataStream.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("delete request failed for data stream %s: %w", dataStream, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		// Data stream doesn't exist, there was nothing to do.
		return nil
	}
	if resp.IsError() {
		return fmt.Errorf("delete request failed for data stream %s: %s", dataStream, resp.String())
	}
	return nil
}

func (r *tester) prepareScenario(ctx context.Context, config *testConfig, stackConfig stack.Config, svcInfo servicedeployer.ServiceInfo) (*scenarioTest, error) {
	serviceOptions := r.createServiceOptions(config.ServiceVariantName, config.Deployer)

	var err error
	var serviceStateData ServiceState
	if r.runSetup {
		err = r.createServiceStateDir()
		if err != nil {
			return nil, fmt.Errorf("failed to create setup services dir: %w", err)
		}
	}
	scenario := scenarioTest{}

	if r.runTearDown || r.runTestsOnly {
		serviceStateData, err = readServiceStateData(r.serviceStateFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read service setup data: %w", err)
		}
	}

	serviceOptions.DeployIndependentAgent = r.runIndependentElasticAgent

	policyTemplateName := config.PolicyTemplate
	if policyTemplateName == "" {
		policyTemplateName, err = findPolicyTemplateForInput(*r.pkgManifest, r.dataStreamManifest, config.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to determine the associated policy_template: %w", err)
		}
	}
	scenario.policyTemplateName = policyTemplateName

	policyTemplate, err := selectPolicyTemplateByName(r.pkgManifest.PolicyTemplates, scenario.policyTemplateName)
	if err != nil {
		return nil, fmt.Errorf("failed to find the selected policy_template: %w", err)
	}
	scenario.policyTemplateInput = policyTemplate.Input

	policyToEnrollOrCurrent, policyToTest, err := r.createOrGetKibanaPolicies(ctx, serviceStateData, stackConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kibana policies: %w", err)
	}

	agentDeployed, agentInfo, err := r.setupAgent(ctx, config, serviceStateData, policyToEnrollOrCurrent)
	if err != nil {
		return nil, err
	}

	scenario.agent = agentDeployed

	if agentDeployed != nil {
		// The Elastic Agent created in `r.setupAgent` needs to be retrieved just after starting it, to ensure
		// it can be removed and unenrolled if the service fails to start.
		// This function must also be called after setting the service (r.setupService), since there are other
		// deployers like custom agents or kubernetes deployer that create new Elastic Agents too that needs to
		// be retrieved too.
		_, err := r.checkEnrolledAgents(ctx, agentInfo, svcInfo)
		if err != nil {
			return nil, fmt.Errorf("can't check enrolled agents: %w", err)
		}
	}

	service, svcInfo, err := r.setupService(ctx, config, serviceOptions, svcInfo, agentInfo, agentDeployed, policyToEnrollOrCurrent, serviceStateData)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if serviceOptions.DeployerName == "" && (errors.Is(err, os.ErrNotExist) || service == nil) {
		// If the service deployer is not defined, it means that the test does not require a service deployer.
		// Just valid when the deployer setting is not defined in the test config.
		logger.Debugf("No service deployer defined for this test")
	}

	// Reload test config with ctx variable substitution.
	config, err = newConfig(config.Path, svcInfo, serviceOptions.Variant)
	if err != nil {
		return nil, fmt.Errorf("unable to reload system test case configuration: %w", err)
	}

	// store the time just before adding the Test Policy, this time will be used to check
	// the agent logs from that time onwards to avoid possible previous errors present in logs
	scenario.startTestTime = time.Now()

	logger.Debug("adding package data stream to test policy...")
	ds, err := createPackageDatastream(*policyToTest, *r.pkgManifest, policyTemplate, r.dataStreamManifest, *config, policyToTest.Namespace)
	if err != nil {
		return nil, fmt.Errorf("could not create package data stream: %w", err)
	}
	if r.runTearDown {
		logger.Debug("Skip adding data stream config to policy")
	} else {
		if err := r.kibanaClient.AddPackageDataStreamToPolicy(ctx, ds); err != nil {
			return nil, fmt.Errorf("could not add data stream config to policy: %w", err)
		}
	}
	scenario.kibanaDataStream = ds

	scenario.indexTemplateName = r.buildIndexTemplateName(ds, config)
	scenario.dataStream = r.buildDataStreamName(scenario.policyTemplateInput, ds, config)

	r.cleanTestScenarioHandler = func(ctx context.Context) error {
		logger.Debugf("Deleting data stream for testing %s", scenario.dataStream)
		err := r.deleteDataStream(ctx, scenario.dataStream)
		if err != nil {
			return fmt.Errorf("failed to delete data stream %s: %w", scenario.dataStream, err)
		}
		return nil
	}

	// While there could be created Elastic Agents within `setupService()` (custom agents and k8s agents),
	// this "checkEnrolledAgents" call must be duplicated here after creating the service too. This will
	// ensure to get the right Enrolled Elastic Agent too.
	agent, err := r.checkEnrolledAgents(ctx, agentInfo, svcInfo)
	if err != nil {
		return nil, fmt.Errorf("can't check enrolled agents: %w", err)
	}

	// FIXME: running per stages does not work when multiple agents are created
	var origPolicy kibana.Policy
	if r.runTearDown {
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
		if r.runSetup {
			// it should be kept the same policy just when system tests are
			// triggered with the flags for running spolicyToAssignDatastreamTestsetup stage (--setup)
			return nil
		}

		// RunTestOnly step (--no-provision) should also reassign back the previous (original) policy
		// even with with independent Elastic Agents, since this step creates a new test policy each execution
		// Moreover, ensure there is no agent service deployer (deprecated) being used
		if scenario.agent != nil && r.runIndependentElasticAgent && !r.runTestsOnly {
			return nil
		}

		logger.Debug("reassigning original policy back to agent...")
		if err := r.kibanaClient.AssignPolicyToAgent(ctx, *agent, origPolicy); err != nil {
			return fmt.Errorf("error reassigning original policy to agent: %w", err)
		}
		return nil
	}

	origAgent := agent
	origLogLevel := ""
	if r.runTearDown {
		logger.Debug("Skip assiging log level debug to agent")
		origLogLevel = serviceStateData.Agent.LocalMetadata.Elastic.Agent.LogLevel
	} else {
		logger.Debug("Set Debug log level to agent")
		origLogLevel = agent.LocalMetadata.Elastic.Agent.LogLevel
		err = r.kibanaClient.SetAgentLogLevel(ctx, agent.ID, "debug")
		if err != nil {
			return nil, fmt.Errorf("error setting log level debug for agent %s: %w", agent.ID, err)
		}
	}
	r.resetAgentLogLevelHandler = func(ctx context.Context) error {
		if r.runTestsOnly || r.runSetup {
			return nil
		}

		// No need to reset agent log level when running independent Elastic Agents
		// since the Elastic Agent is going to be removed/uninstalled
		// Morevoer, ensure there is no agent service deployer (deprecated) being used
		if scenario.agent != nil && r.runIndependentElasticAgent {
			return nil
		}

		logger.Debugf("reassigning original log level %q back to agent...", origLogLevel)

		if err := r.kibanaClient.SetAgentLogLevel(ctx, agent.ID, origLogLevel); err != nil {
			return fmt.Errorf("error reassigning original log level to agent: %w", err)
		}
		return nil
	}

	if r.runTearDown {
		logger.Debug("Skip assigning package data stream to agent")
	} else {
		policyWithDataStream, err := r.kibanaClient.GetPolicy(ctx, policyToTest.ID)
		if err != nil {
			return nil, fmt.Errorf("could not read the policy with data stream: %w", err)
		}

		logger.Debug("assigning package data stream to agent...")
		if err := r.kibanaClient.AssignPolicyToAgent(ctx, *agent, *policyWithDataStream); err != nil {
			return nil, fmt.Errorf("could not assign policy to agent: %w", err)
		}
	}

	// Signal to the service that the agent is ready (policy is assigned).
	if service != nil && config.ServiceNotifySignal != "" {
		if err = service.Signal(ctx, config.ServiceNotifySignal); err != nil {
			return nil, fmt.Errorf("failed to notify test service: %w", err)
		}
	}

	if r.runTearDown {
		return &scenario, nil
	}

	hits, waitErr := r.waitForDocs(ctx, config, scenario.dataStream)

	// before checking "waitErr" error , it is necessary to check if the service has finished with error
	// to report it as a test case failed
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

	// Get deprecation warnings after ensuring that there are ingested docs and thus the
	// data stream exists.
	scenario.deprecationWarnings, err = r.getDeprecationWarnings(ctx, scenario.dataStream)
	if err != nil {
		return nil, fmt.Errorf("failed to get deprecation warnings for data stream %s: %w", scenario.dataStream, err)
	}
	logger.Debugf("Found %d deprecation warnings for data stream %s", len(scenario.deprecationWarnings), scenario.dataStream)

	logger.Debugf("Check whether or not synthetic source mode is enabled (data stream %s)...", scenario.dataStream)
	scenario.syntheticEnabled, err = isSyntheticSourceModeEnabled(ctx, r.esAPI, scenario.dataStream)
	if err != nil {
		return nil, fmt.Errorf("failed to check if synthetic source mode is enabled for data stream %s: %w", scenario.dataStream, err)
	}
	logger.Debugf("Data stream %s has synthetic source mode enabled: %t", scenario.dataStream, scenario.syntheticEnabled)

	scenario.docs = hits.getDocs(scenario.syntheticEnabled)
	scenario.ignoredFields = hits.IgnoredFields
	scenario.degradedDocs = hits.DegradedDocs

	if r.runSetup {
		opts := scenarioStateOpts{
			origPolicy:    &origPolicy,
			enrollPolicy:  policyToEnrollOrCurrent,
			currentPolicy: policyToTest,
			config:        config,
			agent:         *origAgent,
			agentInfo:     agentInfo,
			svcInfo:       svcInfo,
		}
		err = writeScenarioState(opts, r.serviceStateFilePath)
		if err != nil {
			return nil, err
		}
	}

	return &scenario, nil
}

// buildIndexTemplateName builds the expected index template name that is installed in Elasticsearch
// when the package data stream is added to the policy.
func (r *tester) buildIndexTemplateName(ds kibana.PackageDataStream, config *testConfig) string {
	dataStreamDataset := getExpectedDatasetForTest(r.pkgManifest.Type, ds.Inputs[0].Streams[0].DataStream.Dataset, config)

	indexTemplateName := fmt.Sprintf(
		"%s-%s",
		ds.Inputs[0].Streams[0].DataStream.Type,
		dataStreamDataset,
	)
	return indexTemplateName
}

func (r *tester) buildDataStreamName(policyTemplateInput string, ds kibana.PackageDataStream, config *testConfig) string {
	dataStreamDataset := getExpectedDatasetForTest(r.pkgManifest.Type, ds.Inputs[0].Streams[0].DataStream.Dataset, config)

	// Input packages using the otel collector input require to add a specific dataset suffix
	if r.pkgManifest.Type == "input" && policyTemplateInput == otelCollectorInputName {
		dataStreamDataset = fmt.Sprintf("%s.%s", dataStreamDataset, otelSuffixDataset)
	}

	dataStreamName := fmt.Sprintf(
		"%s-%s-%s",
		ds.Inputs[0].Streams[0].DataStream.Type,
		dataStreamDataset,
		ds.Namespace,
	)
	return dataStreamName
}

func getExpectedDatasetForTest(pkgType, dataset string, config *testConfig) string {
	if pkgType == "input" {
		// Input packages can set `data_stream.dataset` by convention to customize the dataset.
		v, _ := config.Vars.GetValue("data_stream.dataset")
		if ds, ok := v.(string); ok && ds != "" {
			return ds
		}
	}
	return dataset
}

// createOrGetKibanaPolicies creates the Kibana policies required for testing.
// It creates two policies, one for enrolling the agent (policyToEnroll) and another one
// for testing purposes (policyToTest) where the package data stream is added.
// In case the tester is running with --teardown or --no-provision flags, then the policies
// are read from the service state file created in the setup stage.
func (r *tester) createOrGetKibanaPolicies(ctx context.Context, serviceStateData ServiceState, stackConfig stack.Config) (*kibana.Policy, *kibana.Policy, error) {
	// Configure package (single data stream) via Fleet APIs.
	testTime := time.Now().Format("20060102T15:04:05Z")
	var policyToTest, policyCurrent, policyToEnroll *kibana.Policy
	var err error

	if r.runTearDown || r.runTestsOnly {
		policyCurrent = &serviceStateData.CurrentPolicy
		policyToEnroll = &serviceStateData.EnrollPolicy
		logger.Debugf("Got current policy from file: %q - %q", policyCurrent.Name, policyCurrent.ID)
	} else {
		// Created a specific Agent Policy to enrolling purposes
		// There are some issues when the stack is running for some time,
		// agents cannot enroll with the default policy
		// This enroll policy must be created even if independent Elastic Agents are not used. Agents created
		// in Kubernetes or Custom Agents require this enroll policy too (service deployer).
		logger.Debug("creating enroll policy...")
		policyEnroll := kibana.Policy{
			Name:        fmt.Sprintf("ep-test-system-enroll-%s-%s-%s-%s-%s", r.testFolder.Package, r.testFolder.DataStream, r.serviceVariant, r.configFileName, testTime),
			Description: fmt.Sprintf("test policy created by elastic-package to enroll agent for data stream %s/%s", r.testFolder.Package, r.testFolder.DataStream),
			Namespace:   common.CreateTestRunID(),
		}

		policyToEnroll, err = r.kibanaClient.CreatePolicy(ctx, policyEnroll)
		if err != nil {
			return nil, nil, fmt.Errorf("could not create test policy: %w", err)
		}
	}

	r.deleteTestPolicyHandler = func(ctx context.Context) error {
		// ensure that policyToEnroll policy gets deleted if the execution receives a signal
		// before creating the test policy
		// This handler is going to be redefined after creating the test policy
		if r.runTestsOnly {
			return nil
		}
		if err := r.kibanaClient.DeletePolicy(ctx, policyToEnroll.ID); err != nil {
			return fmt.Errorf("error cleaning up test policy: %w", err)
		}
		return nil
	}

	if r.runTearDown {
		// required to assign the policy stored in the service state file
		// so data stream related to this Agent Policy can be obtained (and deleted)
		// in the cleanTestScenarioHandler handler
		policyToTest = policyCurrent
	} else {
		// Create a specific Agent Policy just for testing this test.
		// This allows us to ensure that the Agent Policy used for testing is
		// assigned to the agent with all the required changes (e.g. Package DataStream)
		logger.Debug("creating test policy...")
		policy := kibana.Policy{
			Name:        fmt.Sprintf("ep-test-system-%s-%s-%s-%s-%s", r.testFolder.Package, r.testFolder.DataStream, r.serviceVariant, r.configFileName, testTime),
			Description: fmt.Sprintf("test policy created by elastic-package test system for data stream %s/%s", r.testFolder.Package, r.testFolder.DataStream),
			Namespace:   common.CreateTestRunID(),
		}
		// Assign the data_output_id to the agent policy to configure the output to logstash. The value is inferred from stack/_static/kibana.yml.tmpl
		// TODO: Migrate from stack.logstash_enabled to the stack config.
		if r.profile.Config("stack.logstash_enabled", "false") == "true" {
			policy.DataOutputID = "fleet-logstash-output"
		}
		if stackConfig.OutputID != "" {
			policy.DataOutputID = stackConfig.OutputID
		}
		policyToTest, err = r.kibanaClient.CreatePolicy(ctx, policy)
		if err != nil {
			return nil, nil, fmt.Errorf("could not create test policy: %w", err)
		}
	}

	r.deleteTestPolicyHandler = func(ctx context.Context) error {
		logger.Debug("deleting test policies...")
		if err := r.kibanaClient.DeletePolicy(ctx, policyToTest.ID); err != nil {
			return fmt.Errorf("error cleaning up test policy: %w", err)
		}
		if r.runTestsOnly {
			return nil
		}
		if err := r.kibanaClient.DeletePolicy(ctx, policyToEnroll.ID); err != nil {
			return fmt.Errorf("error cleaning up test policy: %w", err)
		}
		return nil
	}

	if r.runTearDown || r.runTestsOnly {
		// required to return "policyCurrent" policy in order to be able select the right agent in `checkEnrolledAgents` when
		// using independent agents or custom/kubernetes agents since policy data is set into `agentInfo` variable`
		return policyCurrent, policyToTest, nil
	}

	// policyToEnroll is used in both independent agents and agents created by servicedeployer (custom or kubernetes agents)
	return policyToEnroll, policyToTest, nil
}

func (r *tester) setupService(ctx context.Context, config *testConfig, serviceOptions servicedeployer.FactoryOptions, svcInfo servicedeployer.ServiceInfo, agentInfo agentdeployer.AgentInfo, agentDeployed agentdeployer.DeployedAgent, policy *kibana.Policy, state ServiceState) (servicedeployer.DeployedService, servicedeployer.ServiceInfo, error) {
	logger.Info("Setting up service...")
	if r.runTearDown || r.runTestsOnly {
		svcInfo.Test.RunID = state.ServiceRunID
		svcInfo.OutputDir = state.ServiceOutputDir
	}

	// By default using agent running in the Elastic stack
	svcInfo.AgentNetworkName = stack.Network(r.profile)
	if agentDeployed != nil {
		svcInfo.AgentNetworkName = agentInfo.NetworkName
	}

	// Set the right folder for logs except for custom agents that are still deployed using "servicedeployer"
	if r.runIndependentElasticAgent && agentDeployed != nil {
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
	if serviceDeployer == nil {
		return nil, svcInfo, nil
	}

	service, err := serviceDeployer.SetUp(ctx, svcInfo)
	if err != nil {
		return nil, svcInfo, fmt.Errorf("could not setup service: %w", err)
	}

	r.shutdownServiceHandler = func(ctx context.Context) error {
		if r.runTestsOnly {
			return nil
		}
		if service == nil {
			return nil
		}
		logger.Info("Tearing down service...")
		if err := service.TearDown(ctx); err != nil {
			return fmt.Errorf("error tearing down service: %w", err)
		}

		return nil
	}

	return service, service.Info(), nil
}

func (r *tester) setupAgent(ctx context.Context, config *testConfig, state ServiceState, policy *kibana.Policy) (agentdeployer.DeployedAgent, agentdeployer.AgentInfo, error) {
	if !r.runIndependentElasticAgent {
		return nil, agentdeployer.AgentInfo{}, nil
	}
	agentRunID := common.CreateTestRunID()
	if r.runTearDown || r.runTestsOnly {
		agentRunID = state.AgentRunID
	}
	logger.Info("Setting up independent Elastic Agent...")
	agentInfo, err := r.createAgentInfo(policy, config, agentRunID)
	if err != nil {
		return nil, agentdeployer.AgentInfo{}, err
	}

	agentOptions := r.createAgentOptions(agentInfo.Policy.Name, config.Deployer)
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
		if r.runTestsOnly {
			return nil
		}
		if agentDeployer == nil {
			return nil
		}
		logger.Info("Tearing down agent...")
		if err := agentDeployed.TearDown(ctx); err != nil {
			return fmt.Errorf("error tearing down agent: %w", err)
		}

		return nil
	}
	return agentDeployed, agentDeployed.Info(), nil
}

func (r *tester) removeServiceStateFile() error {
	err := os.Remove(r.serviceStateFilePath)
	if err != nil {
		return fmt.Errorf("failed to remove file %q: %w", r.serviceStateFilePath, err)
	}
	return nil
}

func (r *tester) createServiceStateDir() error {
	dirPath := filepath.Dir(r.serviceStateFilePath)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("mkdir failed (path: %s): %w", dirPath, err)
	}
	return nil
}

func (r *tester) waitForDocs(ctx context.Context, config *testConfig, dataStream string) (*hits, error) {
	// Use custom timeout if the service can't collect data immediately.
	waitForDataTimeout := waitForDataDefaultTimeout
	if config.WaitForDataTimeout > 0 {
		waitForDataTimeout = config.WaitForDataTimeout
	}

	if config.Assert.HitCount > elasticsearchQuerySize {
		return nil, fmt.Errorf("invalid value for assert.hit_count (%d): it must be lower of the maximum query size (%d)", config.Assert.HitCount, elasticsearchQuerySize)
	}

	if config.Assert.MinCount > elasticsearchQuerySize {
		return nil, fmt.Errorf("invalid value for assert.min_count (%d): it must be lower of the maximum query size (%d)", config.Assert.MinCount, elasticsearchQuerySize)
	}

	// (TODO in future) Optionally exercise service to generate load.
	logger.Debugf("checking for expected data in data stream (%s)...", waitForDataTimeout)
	var hits *hits
	oldHits := 0
	foundFields := map[string]any{}
	passed, waitErr := wait.UntilTrue(ctx, func(ctx context.Context) (bool, error) {
		var err error
		hits, err = r.getDocs(ctx, dataStream)
		if err != nil {
			return false, err
		}

		defer func() {
			oldHits = hits.size()
		}()

		assertHitCount := func() bool {
			if config.Assert.HitCount == 0 {
				// not enabled
				return true
			}
			if hits.size() < config.Assert.HitCount {
				return false
			}

			ret := hits.size() == oldHits
			if !ret {
				time.Sleep(4 * time.Second)
			}

			return ret
		}()

		assertFieldsPresent := func() bool {
			if len(config.Assert.FieldsPresent) == 0 {
				// not enabled
				return true
			}
			if hits.size() == 0 {
				// At least there should be one document ingested
				return false
			}
			for _, f := range config.Assert.FieldsPresent {
				if _, found := foundFields[f]; found {
					continue
				}
				found := false
				for _, d := range hits.Fields {
					if _, err := d.GetValue(f); err == nil {
						found = true
						break
					}
				}
				if !found {
					return false
				}
				logger.Debugf("Found field %q in hits", f)
				foundFields[f] = struct{}{}
			}
			return true
		}()

		assertMinCount := func() bool {
			if config.Assert.MinCount > 0 {
				return hits.size() >= config.Assert.MinCount
			}
			// By default at least one document
			return hits.size() > 0
		}()

		return assertFieldsPresent && assertMinCount && assertHitCount, nil
	}, 1*time.Second, waitForDataTimeout)

	if waitErr != nil {
		return nil, waitErr
	}

	if !passed {
		return nil, testrunner.ErrTestCaseFailed{Reason: fmt.Sprintf("could not find the expected hits in %s data stream", dataStream)}
	}

	return hits, nil
}

func (r *tester) validateTestScenario(ctx context.Context, result *testrunner.ResultComposer, scenario *scenarioTest, config *testConfig) ([]testrunner.TestResult, error) {
	logger.Info("Validating test case...")
	expectedDatasets, err := r.expectedDatasets(scenario, config)
	if err != nil {
		return nil, err
	}

	if r.isTestUsingOTELCollectorInput(scenario.policyTemplateInput) {
		logger.Warn("Validation for packages using OpenTelemetry Collector input is experimental")
	}

	fieldsValidator, err := fields.CreateValidatorForDirectory(r.dataStreamPath,
		fields.WithSpecVersion(r.pkgManifest.SpecVersion),
		fields.WithNumericKeywordFields(config.NumericKeywordFields),
		fields.WithStringNumberFields(config.StringNumberFields),
		fields.WithExpectedDatasets(expectedDatasets),
		fields.WithEnabledImportAllECSSChema(true),
		fields.WithDisableNormalization(scenario.syntheticEnabled),
		// When using the OTEL collector input, just a subset of validations are performed (e.g. check expected datasets)
		fields.WithOTELValidation(r.isTestUsingOTELCollectorInput(scenario.policyTemplateInput)),
	)
	if err != nil {
		return result.WithErrorf("creating fields validator for data stream failed (path: %s): %w", r.dataStreamPath, err)
	}

	if errs := validateFields(scenario.docs, fieldsValidator); len(errs) > 0 {
		return result.WithError(testrunner.ErrTestCaseFailed{
			Reason:  fmt.Sprintf("one or more errors found in documents stored in %s data stream", scenario.dataStream),
			Details: errs.Error(),
		})
	}

	if !r.isTestUsingOTELCollectorInput(scenario.policyTemplateInput) && r.fieldValidationMethod == mappingsMethod {
		logger.Debug("Performing validation based on mappings")
		exceptionFields := listExceptionFields(scenario.docs, fieldsValidator)

		mappingsValidator, err := fields.CreateValidatorForMappings(r.esClient,
			fields.WithMappingValidatorFallbackSchema(fieldsValidator.Schema),
			fields.WithMappingValidatorIndexTemplate(scenario.indexTemplateName),
			fields.WithMappingValidatorDataStream(scenario.dataStream),
			fields.WithMappingValidatorExceptionFields(exceptionFields),
		)
		if err != nil {
			return result.WithErrorf("creating mappings validator for data stream failed (data stream: %s): %w", scenario.dataStream, err)
		}

		if errs := validateMappings(ctx, mappingsValidator); len(errs) > 0 {
			return result.WithError(testrunner.ErrTestCaseFailed{
				Reason:  fmt.Sprintf("one or more errors found in mappings in %s index template", scenario.indexTemplateName),
				Details: errs.Error(),
			})
		}
	}

	stackVersion, err := semver.NewVersion(r.stackVersion.Number)
	if err != nil {
		return result.WithErrorf("failed to parse stack version: %w", err)
	}

	err = validateIgnoredFields(stackVersion, scenario, config)
	if err != nil {
		return result.WithError(err)
	}

	docs := scenario.docs
	if scenario.syntheticEnabled {
		docs, err = fieldsValidator.SanitizeSyntheticSourceDocs(scenario.docs)
		if err != nil {
			results, _ := result.WithErrorf("failed to sanitize synthetic source docs: %w", err)
			return results, nil
		}
	}

	specVersion, err := semver.NewVersion(r.pkgManifest.SpecVersion)
	if err != nil {
		return result.WithErrorf("failed to parse format version %q: %w", r.pkgManifest.SpecVersion, err)
	}

	// Write sample events file from first doc, if requested
	if err := r.generateTestResultFile(docs, *specVersion); err != nil {
		return result.WithError(err)
	}

	// Check Hit Count within docs, if 0 then it has not been specified
	if assertionPass, message := assertHitCount(config.Assert.HitCount, docs); !assertionPass {
		result.FailureMsg = message
	}

	// Check transforms if present
	if err := r.checkTransforms(ctx, config, r.pkgManifest, scenario.dataStream, scenario.policyTemplateInput, scenario.syntheticEnabled); err != nil {
		results, _ := result.WithError(err)
		return results, nil
	}

	if scenario.agent != nil {
		logResults, err := r.checkNewAgentLogs(ctx, scenario.agent, scenario.startTestTime, errorPatterns, config.Name())
		if err != nil {
			return result.WithError(err)
		}
		if len(logResults) > 0 {
			return logResults, nil
		}
	}

	if results := r.checkDeprecationWarnings(stackVersion, scenario.deprecationWarnings, config.Name()); len(results) > 0 {
		return results, nil
	}

	if r.withCoverage {
		coverage, err := r.generateCoverageReport(result.CoveragePackageName())
		if err != nil {
			return result.WithErrorf("coverage report generation failed: %w", err)
		}
		result = result.WithCoverage(coverage)
	}

	return result.WithSuccess()
}

func (r *tester) expectedDatasets(scenario *scenarioTest, config *testConfig) ([]string, error) {
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
		// get dataset directly from package policy added when preparing the scenario
		expectedDataset := scenario.kibanaDataStream.Inputs[0].Streams[0].DataStream.Dataset
		if r.pkgManifest.Type == "input" {
			if scenario.policyTemplateInput == otelCollectorInputName {
				// Input packages whose input is `otelcol` must add the `.otel` suffix
				// Example: httpcheck.metrics.otel
				expectedDataset += "." + otelSuffixDataset
			}
		}
		expectedDatasets = []string{expectedDataset}
	}
	if r.pkgManifest.Type == "input" {
		v, _ := config.Vars.GetValue("data_stream.dataset")
		if dataset, ok := v.(string); ok && dataset != "" {
			if scenario.policyTemplateInput == otelCollectorInputName {
				// Input packages whose input is `otelcol` must add the `.otel` suffix
				// Example: httpcheck.metrics.otel
				dataset += "." + otelSuffixDataset
			}
			expectedDatasets = append(expectedDatasets, dataset)
		}
	}

	return expectedDatasets, nil
}

func (r *tester) runTest(ctx context.Context, config *testConfig, stackConfig stack.Config, svcInfo servicedeployer.ServiceInfo) ([]testrunner.TestResult, error) {
	result := r.newResult(config.Name())

	if skip := testrunner.AnySkipConfig(config.Skip, r.globalTestConfig.Skip); skip != nil {
		logger.Warnf("skipping %s test for %s/%s: %s (details: %s)",
			TestType, r.testFolder.Package, r.testFolder.DataStream,
			skip.Reason, skip.Link)
		return result.WithSkip(skip)
	}

	if r.testFolder.DataStream != "" {
		logger.Infof("Running test for data_stream %q with configuration '%s'", r.testFolder.DataStream, config.Name())
	} else {
		logger.Infof("Running test with configuration '%s'", config.Name())
	}

	scenario, err := r.prepareScenario(ctx, config, stackConfig, svcInfo)
	if err != nil {
		// Known issue: do not include this as part of the xUnit results
		// Example: https://buildkite.com/elastic/integrations/builds/22313#01950431-67a5-4544-a720-6047f5de481b/706-2459
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) && pathErr.Op == "fork/exec" && pathErr.Path == "/usr/bin/docker" {
			return result.WithError(err)
		}
		// report all other errors as error entries in the xUnit file
		results, _ := result.WithError(err)
		return results, nil
	}

	if dump, ok := os.LookupEnv(dumpScenarioDocsEnv); ok && dump != "" {
		err := dumpScenarioDocs(scenario.docs)
		if err != nil {
			return nil, fmt.Errorf("failed to dump scenario docs: %w", err)
		}
	}

	return r.validateTestScenario(ctx, result, scenario, config)
}

func (r *tester) isTestUsingOTELCollectorInput(policyTemplateInput string) bool {
	// Just supported for input packages currently
	if r.pkgManifest.Type != "input" {
		return false
	}

	if policyTemplateInput != otelCollectorInputName {
		return false
	}

	return true
}

func dumpScenarioDocs(docs any) error {
	timestamp := time.Now().Format("20060102150405")
	path := filepath.Join(os.TempDir(), fmt.Sprintf("elastic-package-test-docs-dump-%s.json", timestamp))
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create dump file: %w", err)
	}
	defer f.Close()

	logger.Infof("Dumping scenario documents to %s", path)

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(docs); err != nil {
		return fmt.Errorf("failed to encode docs: %w", err)
	}
	return nil
}

func (r *tester) checkEnrolledAgents(ctx context.Context, agentInfo agentdeployer.AgentInfo, svcInfo servicedeployer.ServiceInfo) (*kibana.Agent, error) {
	var agents []kibana.Agent

	enrolled, err := wait.UntilTrue(ctx, func(ctx context.Context) (bool, error) {
		allAgents, err := r.kibanaClient.ListAgents(ctx)
		if err != nil {
			return false, fmt.Errorf("could not list agents: %w", err)
		}

		if r.runIndependentElasticAgent {
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

	agent := agents[0]
	logger.Debugf("Selected enrolled agent %q", agent.ID)

	r.removeAgentHandler = func(ctx context.Context) error {
		if r.runTestsOnly {
			return nil
		}
		// When not using independent agents, service deployers like kubernetes or custom agents create new Elastic Agent
		if !r.runIndependentElasticAgent && !svcInfo.Agent.Independent {
			return nil
		}
		logger.Debug("removing agent...")
		err := r.kibanaClient.RemoveAgent(ctx, agent)
		if err != nil {
			return fmt.Errorf("failed to remove agent %q: %w", agent.ID, err)
		}
		return nil
	}

	return &agent, nil
}

func createPackageDatastream(
	kibanaPolicy kibana.Policy,
	pkg packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	ds *packages.DataStreamManifest,
	config testConfig,
	suffix string,
) (kibana.PackageDataStream, error) {
	if pkg.Type == "input" {
		return createInputPackageDatastream(kibanaPolicy, pkg, policyTemplate, config, suffix), nil
	}
	if ds == nil {
		return kibana.PackageDataStream{}, fmt.Errorf("data stream manifest is required for integration packages")
	}
	return createIntegrationPackageDatastream(kibanaPolicy, pkg, policyTemplate, *ds, config, suffix), nil
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

	dataset := fmt.Sprintf("%s.%s", pkg.Name, ds.Name)
	if len(ds.Dataset) > 0 {
		dataset = ds.Dataset
	}
	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, pkg.Name, ds.Name),
			Enabled: true,
			DataStream: kibana.DataStream{
				Type:    ds.Type,
				Dataset: dataset,
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
			Type:           policyTemplate.Input,
		},
	}

	dataset := fmt.Sprintf("%s.%s", pkg.Name, policyTemplate.Name)
	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", policyTemplate.Input, pkg.Name, policyTemplate.Name),
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

// findPolicyTemplateForInput returns the name of the policy_template that
// applies to the input under test. An error is returned if no policy template
// matches or if multiple policy templates match and the response is ambiguous.
func findPolicyTemplateForInput(pkg packages.PackageManifest, ds *packages.DataStreamManifest, inputName string) (string, error) {
	if pkg.Type == "input" {
		return findPolicyTemplateForInputPackage(pkg, inputName)
	}
	if ds == nil {
		return "", errors.New("data stream must be specified for integration packages")
	}
	return findPolicyTemplateForDataStream(pkg, *ds, inputName)
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

func (r *tester) checkTransforms(ctx context.Context, config *testConfig, pkgManifest *packages.PackageManifest, dataStream, policyTemplateInput string, syntheticEnabled bool) error {
	if config.SkipTransformValidation {
		return nil
	}

	transforms, err := packages.ReadTransformsFromPackageRoot(r.packageRootPath)
	if err != nil {
		return fmt.Errorf("loading transforms for package failed (root: %s): %w", r.packageRootPath, err)
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
			// It cannot be used "ds.Inputs[0].Streams[0].DataStream.Type" since Fleet
			// always create the transform with the prefix "logs-"
			// https://github.com/elastic/kibana/blob/eed02b930ad332ad7261a0a4dff521e36021fb31/x-pack/platform/plugins/shared/fleet/server/services/epm/elasticsearch/transform/install.ts#L855
			"logs",
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
			fields.WithDisableNormalization(syntheticEnabled),
			// When using the OTEL collector input, just a subset of validations are performed (e.g. check expected datasets)
			fields.WithOTELValidation(r.isTestUsingOTELCollectorInput(policyTemplateInput)),
		)
		if err != nil {
			return fmt.Errorf("creating fields validator for data stream failed (path: %s): %w", transformRootPath, err)
		}
		if errs := validateFields(transformDocs, fieldsValidator); len(errs) > 0 {
			return testrunner.ErrTestCaseFailed{
				Reason:  fmt.Sprintf("errors found in documents of preview for transform %s for data stream %s", transformId, dataStream),
				Details: errs.Error(),
			}
		}
	}

	return nil
}

func (r *tester) getTransformId(ctx context.Context, transformPattern string) (string, error) {
	resp, err := r.esAPI.TransformGetTransform(
		r.esAPI.TransformGetTransform.WithContext(ctx),
		r.esAPI.TransformGetTransform.WithTransformID(transformPattern),
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

func (r *tester) previewTransform(ctx context.Context, transformId string) ([]common.MapStr, error) {
	resp, err := r.esAPI.TransformPreviewTransform(
		r.esAPI.TransformPreviewTransform.WithContext(ctx),
		r.esAPI.TransformPreviewTransform.WithTransformID(transformId),
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

	err = os.WriteFile(filepath.Join(path, "sample_event.json"), append(body, '\n'), 0644)
	if err != nil {
		return fmt.Errorf("writing sample event failed: %w", err)
	}

	return nil
}

func validateFields(docs []common.MapStr, fieldsValidator *fields.Validator) multierror.Error {
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
		return multiErr.Unique()
	}
	return nil
}

func listExceptionFields(docs []common.MapStr, fieldsValidator *fields.Validator) []string {
	var allFields []string
	visited := make(map[string]any)
	for _, doc := range docs {
		fields := fieldsValidator.ListExceptionFields(doc)
		for _, f := range fields {
			if _, ok := visited[f]; ok {
				continue
			}
			visited[f] = struct{}{}
			allFields = append(allFields, f)
		}
	}

	logger.Tracef("Fields to be skipped validation: %s", strings.Join(allFields, ","))
	return allFields
}

func validateIgnoredFields(stackVersion *semver.Version, scenario *scenarioTest, config *testConfig) error {
	skipIgnoredFields := append([]string(nil), config.SkipIgnoredFields...)
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

		return testrunner.ErrTestCaseFailed{
			Reason:  "found ignored fields in data stream",
			Details: fmt.Sprintf("found ignored fields in data stream %s: %v. Affected documents: %s", scenario.dataStream, ignoredFields, degradedDocsJSON),
		}
	}

	return nil
}

func validateMappings(ctx context.Context, mappingsValidator *fields.MappingValidator) multierror.Error {
	multiErr := mappingsValidator.ValidateIndexMappings(ctx)
	if len(multiErr) > 0 {
		return multiErr.Unique()
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

func (r *tester) generateTestResultFile(docs []common.MapStr, specVersion semver.Version) error {
	if !r.generateTestResult {
		return nil
	}

	rootPath := r.packageRootPath
	if ds := r.testFolder.DataStream; ds != "" {
		rootPath = filepath.Join(rootPath, "data_stream", ds)
	}

	if err := writeSampleEvent(rootPath, docs[0], specVersion); err != nil {
		return fmt.Errorf("failed to write sample event file: %w", err)
	}

	return nil
}

func (r *tester) checkNewAgentLogs(ctx context.Context, agent agentdeployer.DeployedAgent, startTesting time.Time, errorPatterns []logsByContainer, configName string) (results []testrunner.TestResult, err error) {
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
				Name:       fmt.Sprintf("(%s logs - %s)", patternsContainer.containerName, configName),
				Package:    r.testFolder.Package,
				DataStream: r.testFolder.DataStream,
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

func (r *tester) checkAgentLogs(dump []stack.DumpResult, startTesting time.Time, errorPatterns []logsByContainer) (results []testrunner.TestResult, err error) {
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
				Package:    r.testFolder.Package,
				DataStream: r.testFolder.DataStream,
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

func (r *tester) anyErrorMessages(logsFilePath string, startTime time.Time, errorPatterns []logsRegexp) error {
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

func (r *tester) generateCoverageReport(pkgName string) (testrunner.CoverageReport, error) {
	dsPattern := "*"
	if r.dataStreamManifest != nil && r.dataStreamManifest.Name != "" {
		dsPattern = r.dataStreamManifest.Name
	}

	// This list of patterns includes patterns for all types of packages. It should not be a problem if some path doesn't exist.
	patterns := []string{
		filepath.Join(r.packageRootPath, "manifest.yml"),
		filepath.Join(r.packageRootPath, "fields", "*.yml"),
		filepath.Join(r.packageRootPath, "data_stream", dsPattern, "manifest.yml"),
		filepath.Join(r.packageRootPath, "data_stream", dsPattern, "fields", "*.yml"),
	}

	return testrunner.GenerateBaseFileCoverageReportGlob(pkgName, patterns, r.coverageType, true)
}
