// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/elastic/elastic-package/internal/corpusgenerator"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters/outputs"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/pipeline"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/system"
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const generateLongDescription = `
*BEWARE*: this command is in beta and it's behaviour may change in the future.
Use this command to generate benchmarks corpus data for a package.
Currently, only data for what we have related assets on https://github.com/elastic/elastic-integration-corpus-generator-tool are supported.
For details on how to run this command, review the [HOWTO guide](./docs/howto/generate_corpus.md).`

const benchLongDescription = `Use this command to run benchmarks on a package. Currently, the following types of benchmarks are available:

#### Pipeline Benchmarks

These benchmarks allow you to benchmark any Ingest Node Pipelines defined by your packages.

For details on how to configure pipeline benchmarks for a package, review the [HOWTO guide](./docs/howto/pipeline_benchmarking.md).

#### System Benchmarks

These benchmarks allow you to benchmark an integration end to end.

For details on how to configure system benchmarks for a package, review the [HOWTO guide](./docs/howto/system_benchmarking.md).`

func setupBenchmarkCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Run benchmarks for the package",
		Long:  benchLongDescription,
	}

	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	pipelineCmd := getPipelineCommand()
	cmd.AddCommand(pipelineCmd)

	systemCmd := getSystemCommand()
	cmd.AddCommand(systemCmd)

	generateCorpusCmd := getGenerateCorpusCommand()
	cmd.AddCommand(generateCorpusCmd)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func getPipelineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Run pipeline benchmarks",
		Long:  "Run pipeline benchmarks for the package",
		RunE:  pipelineCommandAction,
	}

	cmd.Flags().BoolP(cobraext.FailOnMissingFlagName, "m", false, cobraext.FailOnMissingFlagDescription)
	cmd.Flags().StringP(cobraext.ReportFormatFlagName, "", string(pipeline.ReportFormatHuman), cobraext.ReportFormatFlagDescription)
	cmd.Flags().StringP(cobraext.ReportOutputFlagName, "", string(outputs.ReportOutputSTDOUT), cobraext.ReportOutputFlagDescription)
	cmd.Flags().StringSliceP(cobraext.DataStreamsFlagName, "d", nil, cobraext.DataStreamsFlagDescription)
	cmd.Flags().BoolP(cobraext.BenchWithTestSamplesFlagName, "", true, cobraext.BenchWithTestSamplesFlagDescription)
	cmd.Flags().IntP(cobraext.BenchNumTopProcsFlagName, "", 10, cobraext.BenchNumTopProcsFlagDescription)

	return cmd
}

func pipelineCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Run pipeline benchmarks for the package")

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
		return fmt.Errorf("locating package root failed: %w", err)
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

	benchFolders, err := pipeline.FindBenchmarkFolders(packageRootPath, dataStreams)
	if err != nil {
		return fmt.Errorf("unable to determine benchmark folder paths: %w", err)
	}

	if useTestSamples {
		testFolders, err := testrunner.FindTestFolders(packageRootPath, dataStreams, testrunner.TestType(pipeline.BenchType))
		if err != nil {
			return fmt.Errorf("unable to determine test folder paths: %w", err)
		}
		benchFolders = append(benchFolders, testFolders...)
	}

	if failOnMissing && len(benchFolders) == 0 {
		if len(dataStreams) > 0 {
			return fmt.Errorf("no pipeline benchmarks found for %s data stream(s)", strings.Join(dataStreams, ","))
		}
		return errors.New("no pipeline benchmarks found")
	}

	esClient, err := elasticsearch.NewClient()
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch client: %w", err)
	}
	err = esClient.CheckHealth(cmd.Context())
	if err != nil {
		return err
	}

	var results []reporters.Reportable
	for idx, folder := range benchFolders {
		opts := pipeline.NewOptions(
			pipeline.WithBenchmarkName(fmt.Sprintf("%s-%d", folder.Package, idx+1)),
			pipeline.WithFolder(folder),
			pipeline.WithPackageRootPath(packageRootPath),
			pipeline.WithESAPI(esClient.API),
			pipeline.WithNumTopProcs(numTopProcs),
			pipeline.WithFormat(reportFormat),
		)
		runner := pipeline.NewPipelineBenchmark(opts)

		r, err := benchrunner.Run(runner)

		if err != nil {
			return fmt.Errorf("error running package pipeline benchmarks: %w", err)
		}

		results = append(results, r)
	}

	if err != nil {
		return fmt.Errorf("error running package pipeline benchmarks: %w", err)
	}

	for _, report := range results {
		if err := reporters.WriteReportable(reporters.Output(reportOutput), report); err != nil {
			return fmt.Errorf("error writing benchmark report: %w", err)
		}
	}

	return nil
}

func getSystemCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Run system benchmarks",
		Long:  "Run system benchmarks for the package",
		RunE:  systemCommandAction,
	}

	cmd.Flags().StringP(cobraext.BenchNameFlagName, "", "", cobraext.BenchNameFlagDescription)
	cmd.Flags().BoolP(cobraext.BenchReindexToMetricstoreFlagName, "", false, cobraext.BenchReindexToMetricstoreFlagDescription)
	cmd.Flags().DurationP(cobraext.BenchMetricsIntervalFlagName, "", time.Second, cobraext.BenchMetricsIntervalFlagDescription)
	cmd.Flags().DurationP(cobraext.DeferCleanupFlagName, "", 0, cobraext.DeferCleanupFlagDescription)

	return cmd
}

func systemCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Run system benchmarks for the package")

	benchName, err := cmd.Flags().GetString(cobraext.BenchNameFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchNameFlagName)
	}

	deferCleanup, err := cmd.Flags().GetDuration(cobraext.DeferCleanupFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.DeferCleanupFlagName)
	}

	metricsInterval, err := cmd.Flags().GetDuration(cobraext.BenchMetricsIntervalFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchMetricsIntervalFlagName)
	}

	dataReindex, err := cmd.Flags().GetBool(cobraext.BenchReindexToMetricstoreFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchReindexToMetricstoreFlagName)
	}

	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return errors.New("package root not found")
	}
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	signal.Enable()

	esClient, err := elasticsearch.NewClient()
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch client: %w", err)
	}
	err = esClient.CheckHealth(cmd.Context())
	if err != nil {
		return err
	}

	kc, err := kibana.NewClient()
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	opts := system.NewOptions(
		system.WithBenchmarkName(benchName),
		system.WithDeferCleanup(deferCleanup),
		system.WithMetricsInterval(metricsInterval),
		system.WithDataReindexing(dataReindex),
		system.WithPackageRootPath(packageRootPath),
		system.WithESAPI(esClient.API),
		system.WithKibanaClient(kc),
		system.WithProfile(profile),
	)
	runner := system.NewSystemBenchmark(opts)

	r, err := benchrunner.Run(runner)
	if err != nil {
		return fmt.Errorf("error running package system benchmarks: %w", err)
	}

	multiReport, ok := r.(reporters.MultiReportable)
	if !ok {
		return fmt.Errorf("system benchmark is expected to return multiple reports")
	}

	// human report will always be the first
	human := multiReport.Split()[0]
	if err := reporters.WriteReportable(reporters.Output(outputs.ReportOutputSTDOUT), human); err != nil {
		return fmt.Errorf("error writing benchmark report: %w", err)
	}

	// file report will always be the second
	file := multiReport.Split()[1]
	if err := reporters.WriteReportable(reporters.Output(outputs.ReportOutputFile), file); err != nil {
		return fmt.Errorf("error writing benchmark report: %w", err)
	}

	return nil
}

func getGenerateCorpusCommand() *cobra.Command {
	generateCorpusCmd := &cobra.Command{
		Use:   "generate-corpus",
		Short: "Generate benchmarks corpus data for the package",
		Long:  generateLongDescription,
		RunE:  generateDataStreamCorpusCommandAction,
	}

	generateCorpusCmd.Flags().StringP(cobraext.PackageFlagName, cobraext.PackageFlagShorthand, "", cobraext.PackageFlagDescription)
	generateCorpusCmd.Flags().StringP(cobraext.GenerateCorpusDataSetFlagName, cobraext.GenerateCorpusDataSetFlagShorthand, "", cobraext.GenerateCorpusDataSetFlagDescription)
	generateCorpusCmd.Flags().StringP(cobraext.GenerateCorpusSizeFlagName, cobraext.GenerateCorpusSizeFlagShorthand, "", cobraext.GenerateCorpusSizeFlagDescription)
	generateCorpusCmd.Flags().StringP(cobraext.GenerateCorpusCommitFlagName, cobraext.GenerateCorpusCommitFlagShorthand, "main", cobraext.GenerateCorpusCommitFlagDescription)
	generateCorpusCmd.Flags().StringP(cobraext.GenerateCorpusRallyTrackOutputDirFlagName, cobraext.GenerateCorpusRallyTrackOutputDirFlagShorthand, "", cobraext.GenerateCorpusRallyTrackOutputDirFlagDescription)

	return generateCorpusCmd
}

func generateDataStreamCorpusCommandAction(cmd *cobra.Command, _ []string) error {
	packageName, err := cmd.Flags().GetString(cobraext.PackageFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageFlagName)
	}

	dataSetName, err := cmd.Flags().GetString(cobraext.GenerateCorpusDataSetFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateCorpusDataSetFlagName)
	}

	totSize, err := cmd.Flags().GetString(cobraext.GenerateCorpusSizeFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateCorpusSizeFlagName)
	}

	totSizeInBytes, err := humanize.ParseBytes(totSize)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateCorpusSizeFlagName)
	}

	commit, err := cmd.Flags().GetString(cobraext.GenerateCorpusCommitFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateCorpusCommitFlagName)
	}

	if len(commit) == 0 {
		commit = "main"
	}

	rallyTrackOutputDir, err := cmd.Flags().GetString(cobraext.GenerateCorpusRallyTrackOutputDirFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.GenerateCorpusRallyTrackOutputDirFlagName)
	}

	genLibClient := corpusgenerator.NewClient(commit)
	generator, err := corpusgenerator.NewGenerator(genLibClient, packageName, dataSetName, totSizeInBytes)
	if err != nil {
		return fmt.Errorf("can't generate benchmarks data corpus for data stream: %w", err)
	}

	// TODO: we need a way to extract the type from the package and dataset, currently hardcode to `metrics`
	dataStream := fmt.Sprintf("metrics-%s.%s-default", packageName, dataSetName)
	err = corpusgenerator.RunGenerator(generator, dataStream, rallyTrackOutputDir)
	if err != nil {
		return fmt.Errorf("can't generate benchmarks data corpus for data stream: %w", err)
	}

	return nil
}
