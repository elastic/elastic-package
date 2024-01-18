// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
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

		cmd.AddCommand(testTypeCmd)
	}

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func testTypeCommandActionFactory(runner testrunner.TestRunner) cobraext.CommandAction {
	testType := runner.Type()
	return func(cmd *cobra.Command, args []string) error {
		cmd.Printf("Run %s tests for the package\n", testType)

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

		signal.Enable()

		var testFolders []testrunner.TestFolder
		if hasDataStreams && runner.CanRunPerDataStream() {
			var dataStreams []string
			// We check for the existence of the data streams flag before trying to
			// parse it because if the root test command is run instead of one of the
			// subcommands of test, the data streams flag will not be defined.
			if cmd.Flags().Lookup(cobraext.DataStreamsFlagName) != nil {
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

		profile, err := cobraext.GetProfileFlag(cmd)
		if err != nil {
			return err
		}

		esClient, err := stack.NewElasticsearchClientFromProfile(profile)
		if err != nil {
			return fmt.Errorf("can't create Elasticsearch client: %w", err)
		}
		err = esClient.CheckHealth(cmd.Context())
		if err != nil {
			return err
		}

		var kibanaClient *kibana.Client
		if testType == "system" || testType == "asset" {
			// pipeline and static tests do not require a kibana client to perform their required operations
			kibanaClient, err = stack.NewKibanaClientFromProfile(profile)
			if err != nil {
				return fmt.Errorf("can't create Kibana client: %w", err)
			}
		}

		var results []testrunner.TestResult
		for _, folder := range testFolders {
			r, err := testrunner.Run(testType, testrunner.TestOptions{
				Profile:            profile,
				TestFolder:         folder,
				PackageRootPath:    packageRootPath,
				GenerateTestResult: generateTestResult,
				API:                esClient.API,
				KibanaClient:       kibanaClient,
				DeferCleanup:       deferCleanup,
				ServiceVariant:     variantFlag,
				WithCoverage:       testCoverage,
				CoverageType:       testCoverageFormat,
			})

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
