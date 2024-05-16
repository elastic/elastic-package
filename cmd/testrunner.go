// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/reporters/formats"
	"github.com/elastic/elastic-package/internal/testrunner/reporters/outputs"
	_ "github.com/elastic/elastic-package/internal/testrunner/runners" // register all test runners
)

const testLongDescription = `Use this command to run tests on a package. Currently, the following types of tests are available:

#### Asset Loading Tests
These tests ensure that all the Elasticsearch and Kibana assets defined by your package get loaded up as expected.

For details on how to run asset loading tests for a package, see the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/asset_testing.md).

#### Pipeline Tests
These tests allow you to exercise any Ingest Node Pipelines defined by your packages.

For details on how to configure pipeline test for a package, review the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/pipeline_testing.md).

#### Static Tests
These tests allow you to verify if all static resources of the package are valid, e.g. if all fields of the sample_event.json are documented.

For details on how to run static tests for a package, see the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/static_testing.md).

#### System Tests
These tests allow you to test a package's ability to ingest data end-to-end.

For details on how to configure amd run system tests, review the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/system_testing.md).`

var enableIndependentAgents = environment.WithElasticPackagePrefix("TEST_ENABLE_INDEPENDENT_AGENT")

func setupTestCommand() *cobraext.Command {
	var testTypeCmdActions []cobraext.CommandAction

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run test suite for the package",
		Long:  testLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Run test suite for the package")

			if len(args) > 0 {
				return fmt.Errorf("unsupported test type: %s", args[0])
			}

			return cobraext.ComposeCommandActions(cmd, args, testTypeCmdActions...)
		},
	}

	cmd.PersistentFlags().BoolP(cobraext.FailOnMissingFlagName, "m", false, cobraext.FailOnMissingFlagDescription)
	cmd.PersistentFlags().BoolP(cobraext.GenerateTestResultFlagName, "g", false, cobraext.GenerateTestResultFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.ReportFormatFlagName, "", string(formats.ReportFormatHuman), cobraext.ReportFormatFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.ReportOutputFlagName, "", string(outputs.ReportOutputSTDOUT), cobraext.ReportOutputFlagDescription)
	cmd.PersistentFlags().BoolP(cobraext.TestCoverageFlagName, "", false, cobraext.TestCoverageFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.TestCoverageFormatFlagName, "", "cobertura", fmt.Sprintf(cobraext.TestCoverageFormatFlagDescription, strings.Join(testrunner.CoverageFormatsList(), ",")))
	cmd.PersistentFlags().DurationP(cobraext.DeferCleanupFlagName, "", 0, cobraext.DeferCleanupFlagDescription)
	cmd.PersistentFlags().String(cobraext.VariantFlagName, "", cobraext.VariantFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	for testType, runner := range testrunner.TestRunners() {
		action := testTypeCommandActionFactory(runner)
		testTypeCmdActions = append(testTypeCmdActions, action)

		testTypeCmd := &cobra.Command{
			Use:   string(testType),
			Short: fmt.Sprintf("Run %s tests", runner.String()),
			Long:  fmt.Sprintf("Run %s tests for the package.", runner.String()),
			Args:  cobra.NoArgs,
			RunE:  action,
		}
		if runner.CanRunPerDataStream() {
			testTypeCmd.Flags().StringSliceP(cobraext.DataStreamsFlagName, "d", nil, cobraext.DataStreamsFlagDescription)
		}

		if runner.CanRunSetupTeardownIndependent() {
			testTypeCmd.Flags().String(cobraext.ConfigFileFlagName, "", cobraext.ConfigFileFlagDescription)
			testTypeCmd.Flags().Bool(cobraext.SetupFlagName, false, cobraext.SetupFlagDescription)
			testTypeCmd.Flags().Bool(cobraext.TearDownFlagName, false, cobraext.TearDownFlagDescription)
			testTypeCmd.Flags().Bool(cobraext.NoProvisionFlagName, false, cobraext.NoProvisionFlagDescription)
			testTypeCmd.MarkFlagsMutuallyExclusive(cobraext.SetupFlagName, cobraext.TearDownFlagName, cobraext.NoProvisionFlagName)
			testTypeCmd.MarkFlagsRequiredTogether(cobraext.ConfigFileFlagName, cobraext.SetupFlagName)

			// config file flag should not be used with tear-down or no-provision flags
			testTypeCmd.MarkFlagsMutuallyExclusive(cobraext.ConfigFileFlagName, cobraext.TearDownFlagName)
			testTypeCmd.MarkFlagsMutuallyExclusive(cobraext.ConfigFileFlagName, cobraext.NoProvisionFlagName)

			// variant flag should not be used with tear-down and no-provision flags
			// cannot be defined here using MarkFlagsMutuallyExclusive as in --config-file
			// this restriction has been managed later in the code when processing the flags
		}

		if runner.CanRunPerDataStream() && runner.CanRunSetupTeardownIndependent() {
			testTypeCmd.MarkFlagsMutuallyExclusive(cobraext.DataStreamsFlagName, cobraext.SetupFlagName)
			testTypeCmd.MarkFlagsMutuallyExclusive(cobraext.DataStreamsFlagName, cobraext.TearDownFlagName)
			testTypeCmd.MarkFlagsMutuallyExclusive(cobraext.DataStreamsFlagName, cobraext.NoProvisionFlagName)
		}

		cmd.AddCommand(testTypeCmd)
	}

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func testTypeCommandActionFactory(runner testrunner.TestRunner) cobraext.CommandAction {
	testType := runner.Type()
	return func(cmd *cobra.Command, args []string) error {
		cmd.Printf("Run %s tests for the package\n", testType)

		profile, err := cobraext.GetProfileFlag(cmd)
		if err != nil {
			return err
		}

		failOnMissing, err := cmd.Flags().GetBool(cobraext.FailOnMissingFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.FailOnMissingFlagName)
		}

		generateTestResult, err := cmd.Flags().GetBool(cobraext.GenerateTestResultFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.GenerateTestResultFlagName)
		}

		reportFormat, err := cmd.Flags().GetString(cobraext.ReportFormatFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.ReportFormatFlagName)
		}

		reportOutput, err := cmd.Flags().GetString(cobraext.ReportOutputFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.ReportOutputFlagName)
		}

		testCoverage, err := cmd.Flags().GetBool(cobraext.TestCoverageFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.TestCoverageFlagName)
		}

		testCoverageFormat, err := cmd.Flags().GetString(cobraext.TestCoverageFormatFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.TestCoverageFormatFlagName)
		}

		if !slices.Contains(testrunner.CoverageFormatsList(), testCoverageFormat) {
			return cobraext.FlagParsingError(fmt.Errorf("coverage format not available: %s", testCoverageFormat), cobraext.TestCoverageFormatFlagName)
		}

		packageRootPath, found, err := packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return fmt.Errorf("locating package root failed: %w", err)
		}

		manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
		if err != nil {
			return fmt.Errorf("reading package manifest failed (path: %s): %w", packageRootPath, err)
		}

		hasDataStreams, err := packageHasDataStreams(manifest)
		if err != nil {
			return fmt.Errorf("cannot determine if package has data streams: %w", err)
		}

		runIndependentElasticAgent := false

		// If the environment variable is present, it always has preference over the root
		// privileges value (if any) defined in the manifest file
		v, ok := os.LookupEnv(enableIndependentAgents)
		if ok {
			runIndependentElasticAgent = strings.ToLower(v) == "true"
		}

		configFileFlag := ""
		runSetup := false
		runTearDown := false
		runTestsOnly := false

		if runner.CanRunSetupTeardownIndependent() && cmd.Flags().Lookup(cobraext.ConfigFileFlagName) != nil {
			// not all test types define these flags
			runSetup, err = cmd.Flags().GetBool(cobraext.SetupFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.SetupFlagName)
			}
			runTearDown, err = cmd.Flags().GetBool(cobraext.TearDownFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.TearDownFlagName)
			}
			runTestsOnly, err = cmd.Flags().GetBool(cobraext.NoProvisionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.NoProvisionFlagName)
			}

			configFileFlag, err = cmd.Flags().GetString(cobraext.ConfigFileFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ConfigFileFlagName)
			}
			if configFileFlag != "" {
				absPath, err := filepath.Abs(configFileFlag)
				if err != nil {
					return fmt.Errorf("cannot obtain the absolute path for config file path: %s", configFileFlag)
				}
				if _, err := os.Stat(absPath); err != nil {
					return fmt.Errorf("can't find config file %s: %w", configFileFlag, err)
				}
				configFileFlag = absPath
			}
		}

		var testFolders []testrunner.TestFolder
		if hasDataStreams && runner.CanRunPerDataStream() {
			var dataStreams []string

			if runner.CanRunSetupTeardownIndependent() && runSetup || runTearDown || runTestsOnly {
				if runTearDown || runTestsOnly {
					configFileFlag, err = readConfigFileFromState(profile.ProfilePath)
					if err != nil {
						return fmt.Errorf("failed to get config file from state: %w", err)
					}
				}
				dataStream := testrunner.ExtractDataStreamFromPath(configFileFlag, packageRootPath)
				dataStreams = append(dataStreams, dataStream)
			} else if cmd.Flags().Lookup(cobraext.DataStreamsFlagName) != nil {
				// We check for the existence of the data streams flag before trying to
				// parse it because if the root test command is run instead of one of the
				// subcommands of test, the data streams flag will not be defined.
				dataStreams, err = cmd.Flags().GetStringSlice(cobraext.DataStreamsFlagName)
				common.TrimStringSlice(dataStreams)
				if err != nil {
					return cobraext.FlagParsingError(err, cobraext.DataStreamsFlagName)
				}

				err = validateDataStreamsFlag(packageRootPath, dataStreams)
				if err != nil {
					return cobraext.FlagParsingError(err, cobraext.DataStreamsFlagName)
				}
			}

			if runner.TestFolderRequired() {
				testFolders, err = testrunner.FindTestFolders(packageRootPath, dataStreams, testType)
				if err != nil {
					return fmt.Errorf("unable to determine test folder paths: %w", err)
				}
			} else {
				testFolders, err = testrunner.AssumeTestFolders(packageRootPath, dataStreams, testType)
				if err != nil {
					return fmt.Errorf("unable to assume test folder paths: %w", err)
				}
			}

			if failOnMissing && len(testFolders) == 0 {
				if len(dataStreams) > 0 {
					return fmt.Errorf("no %s tests found for %s data stream(s)", testType, strings.Join(dataStreams, ","))
				}
				return fmt.Errorf("no %s tests found", testType)
			}
		} else {
			_, pkg := filepath.Split(packageRootPath)

			if runner.TestFolderRequired() {
				testFolders, err = testrunner.FindTestFolders(packageRootPath, nil, testType)
				if err != nil {
					return fmt.Errorf("unable to determine test folder paths: %w", err)
				}
				if failOnMissing && len(testFolders) == 0 {
					return fmt.Errorf("no %s tests found", testType)
				}
			} else {
				testFolders = []testrunner.TestFolder{
					{
						Package: pkg,
					},
				}
			}
		}

		deferCleanup, err := cmd.Flags().GetDuration(cobraext.DeferCleanupFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.DeferCleanupFlagName)
		}

		variantFlag, _ := cmd.Flags().GetString(cobraext.VariantFlagName)

		ctx, stop := signal.Enable(cmd.Context(), logger.Info)
		defer stop()

		var esAPI *elasticsearch.API
		if testType != "static" {
			// static tests do not need a running Elasticsearch
			esClient, err := stack.NewElasticsearchClientFromProfile(profile)
			if err != nil {
				return fmt.Errorf("can't create Elasticsearch client: %w", err)
			}
			err = esClient.CheckHealth(ctx)
			if err != nil {
				return err
			}
			esAPI = esClient.API
		}

		var kibanaClient *kibana.Client
		if testType == "system" || testType == "asset" {
			// pipeline and static tests do not require a kibana client to perform their required operations
			kibanaClient, err = stack.NewKibanaClientFromProfile(profile)
			if err != nil {
				return fmt.Errorf("can't create Kibana client: %w", err)
			}
		}

		if runTearDown || runTestsOnly {
			if variantFlag != "" {
				return fmt.Errorf("variant flag cannot be set with --tear-down or --no-provision")
			}
		}

		if runSetup || runTearDown || runTestsOnly {
			// variant flag is not checked here since there are packages that do not have variants
			if len(testFolders) != 1 {
				return fmt.Errorf("wrong number of test folders (expected 1): %d", len(testFolders))
			}

			fmt.Printf("Running tests per stages (technical preview)\n")
		}

		var results []testrunner.TestResult
		for _, folder := range testFolders {
			r, err := testrunner.Run(ctx, testType, testrunner.TestOptions{
				Profile:                    profile,
				TestFolder:                 folder,
				PackageRootPath:            packageRootPath,
				GenerateTestResult:         generateTestResult,
				API:                        esAPI,
				KibanaClient:               kibanaClient,
				DeferCleanup:               deferCleanup,
				ServiceVariant:             variantFlag,
				WithCoverage:               testCoverage,
				CoverageType:               testCoverageFormat,
				ConfigFilePath:             configFileFlag,
				RunSetup:                   runSetup,
				RunTearDown:                runTearDown,
				RunTestsOnly:               runTestsOnly,
				RunIndependentElasticAgent: runIndependentElasticAgent,
			})

			// Results must be appended even if there is an error, since there could be
			// tests (e.g. system tests) that return both error and results.
			results = append(results, r...)

			if err != nil {
				return fmt.Errorf("error running package %s tests: %w", testType, err)
			}

		}

		format := testrunner.TestReportFormat(reportFormat)
		report, err := testrunner.FormatReport(format, results)
		if err != nil {
			return fmt.Errorf("error formatting test report: %w", err)
		}

		if err := testrunner.WriteReport(manifest.Name, testrunner.TestReportOutput(reportOutput), report, format); err != nil {
			return fmt.Errorf("error writing test report: %w", err)
		}

		if testCoverage {
			err := testrunner.WriteCoverage(packageRootPath, manifest.Name, manifest.Type, runner.Type(), results, testCoverageFormat)
			if err != nil {
				return fmt.Errorf("error writing test coverage: %w", err)
			}
		}

		// Check if there is any error or failure reported
		for _, r := range results {
			if r.ErrorMsg != "" || r.FailureMsg != "" {
				return errors.New("one or more test cases failed")
			}
		}
		return nil
	}
}

func readConfigFileFromState(profilePath string) (string, error) {
	type stateData struct {
		ConfigFilePath string `json:"config_file_path"`
	}
	var serviceStateData stateData
	setupDataPath := filepath.Join(testrunner.StateFolderPath(profilePath), testrunner.ServiceStateFileName)
	fmt.Printf("Reading service state data from file: %s\n", setupDataPath)
	contents, err := os.ReadFile(setupDataPath)
	if err != nil {
		return "", fmt.Errorf("failed to read service state data %q: %w", setupDataPath, err)
	}
	err = json.Unmarshal(contents, &serviceStateData)
	if err != nil {
		return "", fmt.Errorf("failed to decode service state data %q: %w", setupDataPath, err)
	}
	return serviceStateData.ConfigFilePath, nil
}

func packageHasDataStreams(manifest *packages.PackageManifest) (bool, error) {
	switch manifest.Type {
	case "integration":
		return true, nil
	case "input":
		return false, nil
	default:
		return false, fmt.Errorf("unexpected package type %q", manifest.Type)
	}
}

func validateDataStreamsFlag(packageRootPath string, dataStreams []string) error {
	for _, dataStream := range dataStreams {
		path := filepath.Join(packageRootPath, "data_stream", dataStream)
		fileInfo, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat directory failed (path: %s): %w", path, err)
		}

		if !fileInfo.IsDir() {
			return fmt.Errorf("data stream must be a directory (path: %s)", path)
		}
	}
	return nil
}
