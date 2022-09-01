// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters/formats"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters/outputs"
	_ "github.com/elastic/elastic-package/internal/benchrunner/runners" // register all test runners
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
)

const benchLongDescription = `Use this command to run benchmarks on a package. Currently, the following types of benchmarks are available:

#### Pipeline Benchmarks
These benchmarks allow you to benchmark any Ingest Node Pipelines defined by your packages.

For details on how to configure pipeline test for a package, review the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/pipeline_benchmarks.md).`

func setupBenchmarkCommand() *cobraext.Command {
	var benchTypeCmdActions []cobraext.CommandAction

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Run benchmarks for the package",
		Long:  benchLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Run benchmarks for the package")

			if len(args) > 0 {
				return fmt.Errorf("unsupported benchmark type: %s", args[0])
			}

			return cobraext.ComposeCommandActions(cmd, args, benchTypeCmdActions...)
		}}

	cmd.PersistentFlags().BoolP(cobraext.FailOnMissingFlagName, "m", false, cobraext.FailOnMissingFlagDescription)
	cmd.PersistentFlags().BoolP(cobraext.GenerateTestResultFlagName, "g", false, cobraext.GenerateTestResultFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.ReportFormatFlagName, "", string(formats.ReportFormatHuman), cobraext.ReportFormatFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.ReportOutputFlagName, "", string(outputs.ReportOutputSTDOUT), cobraext.ReportOutputFlagDescription)
	cmd.PersistentFlags().BoolP(cobraext.TestCoverageFlagName, "", false, cobraext.TestCoverageFlagDescription)
	cmd.PersistentFlags().IntP(cobraext.TestBenchCountFlagName, "", 1000, cobraext.TestBenchCountFlagDescription)
	cmd.PersistentFlags().DurationP(cobraext.TestPerfDurationFlagName, "", time.Duration(0), cobraext.TestPerfDurationFlagDescription)
	cmd.PersistentFlags().DurationP(cobraext.DeferCleanupFlagName, "", 0, cobraext.DeferCleanupFlagDescription)
	cmd.PersistentFlags().String(cobraext.VariantFlagName, "", cobraext.VariantFlagDescription)

	for benchType, runner := range benchrunner.TestRunners() {
		action := benchTypeCommandActionFactory(runner)
		benchTypeCmdActions = append(benchTypeCmdActions, action)

		benchTypeCmd := &cobra.Command{
			Use:   string(benchType),
			Short: fmt.Sprintf("Run %s benchmarks", runner.String()),
			Long:  fmt.Sprintf("Run %s benchmarks for the package.", runner.String()),
			RunE:  action,
		}

		if runner.CanRunPerDataStream() {
			benchTypeCmd.Flags().StringSliceP(cobraext.DataStreamsFlagName, "d", nil, cobraext.DataStreamsFlagDescription)
		}

		cmd.AddCommand(benchTypeCmd)
	}

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func benchTypeCommandActionFactory(runner benchrunner.TestRunner) cobraext.CommandAction {
	benchType := runner.Type()
	return func(cmd *cobra.Command, args []string) error {
		cmd.Printf("Run %s tests for the package\n", benchType)

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

		testBenchCount, err := cmd.Flags().GetInt(cobraext.TestBenchCountFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.TestBenchCountFlagName)
		}

		testBenchDur, err := cmd.Flags().GetDuration(cobraext.TestPerfDurationFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.TestBenchCountFlagDescription)
		}

		packageRootPath, found, err := packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return errors.Wrap(err, "locating package root failed")
		}

		signal.Enable()

		var testFolders []benchrunner.TestFolder
		if runner.CanRunPerDataStream() {
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
				testFolders, err = benchrunner.FindTestFolders(packageRootPath, dataStreams, benchType)
				if err != nil {
					return errors.Wrap(err, "unable to determine test folder paths")
				}
			} else {
				testFolders, err = benchrunner.AssumeTestFolders(packageRootPath, dataStreams, benchType)
				if err != nil {
					return errors.Wrap(err, "unable to assume test folder paths")
				}
			}

			if failOnMissing && len(testFolders) == 0 {
				if len(dataStreams) > 0 {
					return fmt.Errorf("no %s tests found for %s data stream(s)", benchType, strings.Join(dataStreams, ","))
				}
				return fmt.Errorf("no %s tests found", benchType)
			}
		} else {
			_, pkg := filepath.Split(packageRootPath)
			testFolders = []benchrunner.TestFolder{
				{
					Package: pkg,
				},
			}
		}

		deferCleanup, err := cmd.Flags().GetDuration(cobraext.DeferCleanupFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.DeferCleanupFlagName)
		}

		variantFlag, _ := cmd.Flags().GetString(cobraext.VariantFlagName)

		esClient, err := elasticsearch.Client()
		if err != nil {
			return errors.Wrap(err, "can't create Elasticsearch client")
		}

		var results []benchrunner.TestResult
		for _, folder := range testFolders {
			r, err := benchrunner.Run(benchType, benchrunner.TestOptions{
				TestFolder:         folder,
				PackageRootPath:    packageRootPath,
				GenerateTestResult: generateTestResult,
				API:                esClient.API,
				DeferCleanup:       deferCleanup,
				ServiceVariant:     variantFlag,
				WithCoverage:       testCoverage,
				Benchmark: benchrunner.BenchmarkConfig{
					NumDocs:  testBenchCount,
					Duration: testBenchDur,
				},
			})

			results = append(results, r...)

			if err != nil {
				return errors.Wrapf(err, "error running package %s tests", benchType)
			}
		}

		format := benchrunner.TestReportFormat(reportFormat)
		testReport, benchReports, err := benchrunner.FormatReport(format, results)
		if err != nil {
			return errors.Wrap(err, "error formatting test report")
		}

		m, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
		if err != nil {
			return errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRootPath)
		}

		if err := benchrunner.WriteReport(m.Name, benchrunner.TestReportOutput(reportOutput), testReport, format, benchrunner.ReportTypeTest); err != nil {
			return errors.Wrap(err, "error writing test report")
		}

		for idx, report := range benchReports {
			if err := benchrunner.WriteReport(fmt.Sprintf("%s-%d", m.Name, idx+1), benchrunner.TestReportOutput(reportOutput), report, format, benchrunner.ReportTypeBench); err != nil {
				return errors.Wrap(err, "error writing benchmark report")
			}
		}
		if testCoverage {
			err := benchrunner.WriteCoverage(packageRootPath, m.Name, runner.Type(), results)
			if err != nil {
				return errors.Wrap(err, "error writing test coverage")
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
