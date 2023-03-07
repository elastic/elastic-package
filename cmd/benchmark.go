// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/elastic/elastic-package/internal/corpusgenerator"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters/formats"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters/outputs"
	_ "github.com/elastic/elastic-package/internal/benchrunner/runners" // register all benchmark runners
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const generateLongDescription = `
BEWARE: this command is in beta and it's behaviour may change in the future.
Use this command to generate benchmarks corpus  data for a package.
Currently, only data for what we have related assets on https://github.com/elastic/elastic-integration-corpus-generator-tool are supported.
`

const benchLongDescription = `Use this command to run benchmarks on a package. Currently, the following types of benchmarks are available:

#### Pipeline Benchmarks

These benchmarks allow you to benchmark any Ingest Node Pipelines defined by your packages.

For details on how to configure pipeline benchmarks for a package, review the [HOWTO guide](./docs/howto/pipeline_benchmarking.md).`

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
	cmd.PersistentFlags().StringP(cobraext.ReportFormatFlagName, "", string(formats.ReportFormatHuman), cobraext.ReportFormatFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.ReportOutputFlagName, "", string(outputs.ReportOutputSTDOUT), cobraext.ReportOutputFlagDescription)
	cmd.PersistentFlags().BoolP(cobraext.BenchWithTestSamplesFlagName, "", true, cobraext.BenchWithTestSamplesFlagDescription)
	cmd.PersistentFlags().IntP(cobraext.BenchNumTopProcsFlagName, "", 10, cobraext.BenchNumTopProcsFlagDescription)
	cmd.PersistentFlags().StringSliceP(cobraext.DataStreamsFlagName, "", nil, cobraext.DataStreamsFlagDescription)

	for benchType, runner := range benchrunner.BenchRunners() {
		action := benchTypeCommandActionFactory(runner)
		benchTypeCmdActions = append(benchTypeCmdActions, action)

		benchTypeCmd := &cobra.Command{
			Use:   string(benchType),
			Short: fmt.Sprintf("Run %s benchmarks", runner.String()),
			Long:  fmt.Sprintf("Run %s benchmarks for the package.", runner.String()),
			RunE:  action,
		}

		benchTypeCmd.Flags().StringSliceP(cobraext.DataStreamsFlagName, "d", nil, cobraext.DataStreamsFlagDescription)

		cmd.AddCommand(benchTypeCmd)
	}

	generateCorpusCmd := getGenerateCorpusCommand()
	cmd.AddCommand(generateCorpusCmd)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func benchTypeCommandActionFactory(runner benchrunner.BenchRunner) cobraext.CommandAction {
	benchType := runner.Type()
	return func(cmd *cobra.Command, args []string) error {
		cmd.Printf("Run %s benchmarks for the package\n", benchType)

		failOnMissing, err := cmd.Flags().GetBool(cobraext.FailOnMissingFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.FailOnMissingFlagName)
		}

		reportFormat, err := cmd.Flags().GetString(cobraext.ReportFormatFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.ReportFormatFlagName)
		}

		reportOutput, err := cmd.Flags().GetString(cobraext.ReportOutputFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.ReportOutputFlagName)
		}

		useTestSamples, err := cmd.Flags().GetBool(cobraext.BenchWithTestSamplesFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.BenchWithTestSamplesFlagName)
		}

		numTopProcs, err := cmd.Flags().GetInt(cobraext.BenchNumTopProcsFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.BenchNumTopProcsFlagName)
		}

		packageRootPath, found, err := packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return errors.Wrap(err, "locating package root failed")
		}

		dataStreams, err := cmd.Flags().GetStringSlice(cobraext.DataStreamsFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.DataStreamsFlagName)
		}

		if len(dataStreams) > 0 {
			common.TrimStringSlice(dataStreams)

			if err := validateDataStreamsFlag(packageRootPath, dataStreams); err != nil {
				return cobraext.FlagParsingError(err, cobraext.DataStreamsFlagName)
			}
		}

		signal.Enable()

		benchFolders, err := benchrunner.FindBenchmarkFolders(packageRootPath, dataStreams, benchType)
		if err != nil {
			return errors.Wrap(err, "unable to determine benchmark folder paths")
		}

		if useTestSamples {
			testFolders, err := testrunner.FindTestFolders(packageRootPath, dataStreams, testrunner.TestType(benchType))
			if err != nil {
				return errors.Wrap(err, "unable to determine test folder paths")
			}
			benchFolders = append(benchFolders, testFolders...)
		}

		if failOnMissing && len(benchFolders) == 0 {
			if len(dataStreams) > 0 {
				return fmt.Errorf("no %s benchmarks found for %s data stream(s)", benchType, strings.Join(dataStreams, ","))
			}
			return fmt.Errorf("no %s benchmarks found", benchType)
		}

		esClient, err := elasticsearch.NewClient()
		if err != nil {
			return errors.Wrap(err, "can't create Elasticsearch client")
		}
		err = esClient.CheckHealth(cmd.Context())
		if err != nil {
			return err
		}

		var results []*benchrunner.Result
		for _, folder := range benchFolders {
			r, err := benchrunner.Run(benchType, benchrunner.BenchOptions{
				Folder:          folder,
				PackageRootPath: packageRootPath,
				API:             esClient.API,
				NumTopProcs:     numTopProcs,
			})

			if err != nil {
				return errors.Wrapf(err, "error running package %s benchmarks", benchType)
			}

			results = append(results, r)
		}

		format := benchrunner.BenchReportFormat(reportFormat)
		benchReports, err := benchrunner.FormatReport(format, results)
		if err != nil {
			return errors.Wrap(err, "error formatting benchmark report")
		}

		m, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
		if err != nil {
			return errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRootPath)
		}

		for idx, report := range benchReports {
			if err := benchrunner.WriteReport(fmt.Sprintf("%s-%d", m.Name, idx+1), benchrunner.BenchReportOutput(reportOutput), report, format); err != nil {
				return errors.Wrap(err, "error writing benchmark report")
			}
		}

		// Check if there is any error or failure reported
		for _, r := range results {
			if r.ErrorMsg != "" {
				return fmt.Errorf("one or more benchmarks failed: %v", r.ErrorMsg)
			}
		}
		return nil
	}
}

func getGenerateCorpusCommand() *cobra.Command {
	generateCorpusCmd := &cobra.Command{
		Use:   "generate-corpus",
		Short: "Generate benchmarks corpus data for the package",
		Long:  generateLongDescription,
		RunE:  generateDataStreamCorpusCommandAction,
	}

	generateCorpusCmd.PersistentFlags().StringP(cobraext.PackageFlagName, cobraext.PackageFlagShorthand, "", cobraext.PackageFlagDescription)
	generateCorpusCmd.PersistentFlags().StringP(cobraext.GenerateCorpusDataStreamFlagName, cobraext.GenerateCorpusDataStreamFlagShorthand, "", cobraext.GenerateCorpusDataStreamFlagDescription)
	generateCorpusCmd.PersistentFlags().StringP(cobraext.GenerateCorpusSizeFlagName, cobraext.GenerateCorpusSizeFlagShorthand, "", cobraext.GenerateCorpusSizeFlagDescription)
	generateCorpusCmd.PersistentFlags().StringP(cobraext.GenerateCorpusCommitFlagName, cobraext.GenerateCorpusCommitFlagShorthand, "main", cobraext.GenerateCorpusCommitFlagDescription)

	return generateCorpusCmd
}

func generateDataStreamCorpusCommandAction(cmd *cobra.Command, _ []string) error {
	packageName, err := cmd.Flags().GetString(cobraext.PackageFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageFlagName)
	}

	dataStreamName, err := cmd.Flags().GetString(cobraext.GenerateCorpusDataStreamFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateCorpusDataStreamFlagName)
	}

	totSize, err := cmd.Flags().GetString(cobraext.GenerateCorpusSizeFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateCorpusSizeFlagName)
	}

	totSizeInBytes, err := humanize.ParseBytes(totSize)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateCorpusSizeFlagName)
	}

	if totSizeInBytes < 0 {
		return cobraext.FlagParsingError(errors.New("provide a positive size vaue"), cobraext.GenerateCorpusSizeFlagName)
	}

	commit, err := cmd.Flags().GetString(cobraext.GenerateCorpusCommitFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateCorpusSizeFlagName)
	}

	if len(commit) == 0 {
		commit = "main"
	}

	generator, err := corpusgenerator.GetGenerator(packageName, dataStreamName, commit, totSizeInBytes)
	if err != nil {
		return errors.Wrap(err, "can't generate benchmarks data corpus for data stream")
	}

	err = corpusgenerator.RunGenerator(generator)
	if err != nil {
		return errors.Wrap(err, "can't generate benchmarks data corpus for data stream")
	}

	return nil
}
