// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters/outputs"
	benchcommon "github.com/elastic/elastic-package/internal/benchrunner/runners/common"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/pipeline"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/rally"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/stream"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/system"
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const benchLongDescription = `Use this command to run benchmarks on a package. Currently, the following types of benchmarks are available:

#### Pipeline Benchmarks

These benchmarks allow you to benchmark any Ingest Node Pipelines defined by your packages.

For details on how to configure pipeline benchmarks for a package, review the [HOWTO guide](./docs/howto/pipeline_benchmarking.md).

#### Rally Benchmarks

These benchmarks allow you to benchmark an integration corpus with rally.

For details on how to configure rally benchmarks for a package, review the [HOWTO guide](./docs/howto/rally_benchmarking.md).

#### Stream Benchmarks

These benchmarks allow you to benchmark ingesting real time data.
You can stream data to a remote ES cluster setting the following environment variables:

` + "```" + `
ELASTIC_PACKAGE_ELASTICSEARCH_HOST=https://my-deployment.es.eu-central-1.aws.foundit.no
ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=elastic
ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=changeme
ELASTIC_PACKAGE_KIBANA_HOST=https://my-deployment.kb.eu-central-1.aws.foundit.no:9243
` + "```" + `

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

	rallyCmd := getRallyCommand()
	cmd.AddCommand(rallyCmd)

	streamCmd := getStreamCommand()
	cmd.AddCommand(streamCmd)

	systemCmd := getSystemCommand()
	cmd.AddCommand(systemCmd)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func getPipelineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Run pipeline benchmarks",
		Long:  "Run pipeline benchmarks for the package",
		Args:  cobra.NoArgs,
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

	packageRootPath, err := packages.FindPackageRoot()
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

	ctx, stop := signal.Enable(cmd.Context(), logger.Info)
	defer stop()

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

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	esClient, err := stack.NewElasticsearchClientFromProfile(profile)
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch client: %w", err)
	}
	err = esClient.CheckHealth(ctx)
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

		r, err := benchrunner.Run(ctx, runner)

		if err != nil {
			return fmt.Errorf("error running package pipeline benchmarks: %w", err)
		}

		results = append(results, r)
	}

	for _, report := range results {
		if err := reporters.WriteReportable(reporters.Output(reportOutput), report); err != nil {
			return fmt.Errorf("error writing benchmark report: %w", err)
		}
	}

	return nil
}

func getRallyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rally",
		Short: "Run rally benchmarks",
		Long:  "Run rally benchmarks for the package (esrally needs to be installed in the path of the system)",
		Args:  cobra.NoArgs,
		RunE:  rallyCommandAction,
	}

	cmd.Flags().StringP(cobraext.BenchNameFlagName, "", "", cobraext.BenchNameFlagDescription)
	cmd.Flags().BoolP(cobraext.BenchReindexToMetricstoreFlagName, "", false, cobraext.BenchReindexToMetricstoreFlagDescription)
	cmd.Flags().DurationP(cobraext.BenchMetricsIntervalFlagName, "", time.Second, cobraext.BenchMetricsIntervalFlagDescription)
	cmd.Flags().DurationP(cobraext.DeferCleanupFlagName, "", 0, cobraext.DeferCleanupFlagDescription)
	cmd.Flags().String(cobraext.VariantFlagName, "", cobraext.VariantFlagDescription)
	cmd.Flags().StringP(cobraext.BenchCorpusRallyTrackOutputDirFlagName, "", "", cobraext.BenchCorpusRallyTrackOutputDirFlagDescription)
	cmd.Flags().BoolP(cobraext.BenchCorpusRallyDryRunFlagName, "", false, cobraext.BenchCorpusRallyDryRunFlagDescription)
	cmd.Flags().StringP(cobraext.BenchCorpusRallyUseCorpusAtPathFlagName, "", "", cobraext.BenchCorpusRallyUseCorpusAtPathFlagDescription)
	cmd.Flags().StringP(cobraext.BenchCorpusRallyPackageFromRegistryFlagName, "", "", cobraext.BenchCorpusRallyPackageFromRegistryFlagDescription)
	cmd.MarkFlagRequired(cobraext.BenchNameFlagName)

	return cmd
}

func rallyCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Run rally benchmarks for the package")

	variant, err := cmd.Flags().GetString(cobraext.VariantFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.VariantFlagName)
	}

	benchName, err := cmd.Flags().GetString(cobraext.BenchNameFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchNameFlagName)
	}

	dataReindex, err := cmd.Flags().GetBool(cobraext.BenchReindexToMetricstoreFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchReindexToMetricstoreFlagName)
	}

	rallyTrackOutputDir, err := cmd.Flags().GetString(cobraext.BenchCorpusRallyTrackOutputDirFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchCorpusRallyTrackOutputDirFlagName)
	}

	rallyDryRun, err := cmd.Flags().GetBool(cobraext.BenchCorpusRallyDryRunFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchCorpusRallyDryRunFlagName)
	}

	corpusAtPath, err := cmd.Flags().GetString(cobraext.BenchCorpusRallyUseCorpusAtPathFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchCorpusRallyUseCorpusAtPathFlagName)
	}

	packageFromRegistry, err := cmd.Flags().GetString(cobraext.BenchCorpusRallyPackageFromRegistryFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchCorpusRallyPackageFromRegistryFlagName)
	}

	packageName, packageVersion, err := getPackageNameAndVersion(packageFromRegistry)
	if err != nil {
		return fmt.Errorf("getting package name and version failed, expected format: <package>-<version>: %w", err)
	}

	var packageRootPath string
	if len(packageName) == 0 {
		packageRootPath, err = packages.FindPackageRoot()
		if err != nil {
			return fmt.Errorf("locating package root failed: %w", err)
		}
	}

	repositoryRoot, err := files.FindRepositoryRoot()
	if err != nil {
		return fmt.Errorf("locating repository root failed: %w", err)
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	ctx, stop := signal.Enable(cmd.Context(), logger.Info)
	defer stop()

	esClient, err := stack.NewElasticsearchClientFromProfile(profile)
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch client: %w", err)
	}
	err = esClient.CheckHealth(ctx)
	if err != nil {
		return err
	}

	kc, err := stack.NewKibanaClientFromProfile(profile)
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	withOpts := []rally.OptionFunc{
		rally.WithVariant(variant),
		rally.WithBenchmarkName(benchName),
		rally.WithDataReindexing(dataReindex),
		rally.WithPackageRootPath(packageRootPath),
		rally.WithESAPI(esClient.API),
		rally.WithKibanaClient(kc),
		rally.WithProfile(profile),
		rally.WithRallyTrackOutputDir(rallyTrackOutputDir),
		rally.WithRallyDryRun(rallyDryRun),
		rally.WithRallyPackageFromRegistry(packageName, packageVersion),
		rally.WithRallyCorpusAtPath(corpusAtPath),
		rally.WithRepositoryRoot(repositoryRoot),
	}

	esMetricsClient, err := initializeESMetricsClient(ctx)
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch metrics client: %w", err)
	}
	if esMetricsClient != nil {
		withOpts = append(withOpts, rally.WithESMetricsAPI(esMetricsClient.API))
	}

	runner := rally.NewRallyBenchmark(rally.NewOptions(withOpts...))

	r, err := benchrunner.Run(ctx, runner)
	if errors.Is(err, rally.ErrDryRun) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("error running package rally benchmarks: %w", err)
	}

	multiReport, ok := r.(reporters.MultiReportable)
	if !ok {
		return fmt.Errorf("rally benchmark is expected to return multiple reports")
	}

	reports := multiReport.Split()
	if len(reports) != 2 {
		return fmt.Errorf("rally benchmark is expected to return a human and a file report")
	}

	// human report will always be the first
	human := reports[0]
	if err := reporters.WriteReportable(reporters.Output(outputs.ReportOutputSTDOUT), human); err != nil {
		return fmt.Errorf("error writing benchmark report: %w", err)
	}

	// file report will always be the second
	file := reports[1]
	if err := reporters.WriteReportable(reporters.Output(outputs.ReportOutputFile), file); err != nil {
		return fmt.Errorf("error writing benchmark report: %w", err)
	}

	return nil
}

func getPackageNameAndVersion(packageFromRegistry string) (string, string, error) {
	if len(packageFromRegistry) == 0 {
		return "", "", nil
	}

	name, version, valid := strings.Cut(packageFromRegistry, "-")
	if !valid || name == "" || version == "" {
		return "", "", fmt.Errorf("package name and version from registry not valid (%s)", packageFromRegistry)
	}

	return name, version, nil
}

func getStreamCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Run stream benchmarks",
		Long:  "Run stream benchmarks for the package",
		Args:  cobra.NoArgs,
		RunE:  streamCommandAction,
	}

	cmd.Flags().StringP(cobraext.BenchNameFlagName, "", "", cobraext.BenchNameFlagDescription)
	cmd.Flags().String(cobraext.VariantFlagName, "", cobraext.VariantFlagDescription)
	cmd.Flags().DurationP(cobraext.BenchStreamBackFillFlagName, "", 15*time.Minute, cobraext.BenchStreamBackFillFlagDescription)
	cmd.Flags().Uint64P(cobraext.BenchStreamEventsPerPeriodFlagName, "", 10, cobraext.BenchStreamEventsPerPeriodFlagDescription)
	cmd.Flags().DurationP(cobraext.BenchStreamPeriodDurationFlagName, "", 10*time.Second, cobraext.BenchStreamPeriodDurationFlagDescription)
	cmd.Flags().BoolP(cobraext.BenchStreamPerformCleanupFlagName, "", false, cobraext.BenchStreamPerformCleanupFlagDescription)
	cmd.Flags().StringP(cobraext.BenchStreamTimestampFieldFlagName, "", "timestamp", cobraext.BenchStreamTimestampFieldFlagDescription)

	return cmd
}

func streamCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Run stream benchmarks for the package")

	variant, err := cmd.Flags().GetString(cobraext.VariantFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.VariantFlagName)
	}

	benchName, err := cmd.Flags().GetString(cobraext.BenchNameFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchNameFlagName)
	}

	backFill, err := cmd.Flags().GetDuration(cobraext.BenchStreamBackFillFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchStreamBackFillFlagName)
	}

	if backFill < 0 {
		return cobraext.FlagParsingError(errors.New("cannot be a negative duration"), cobraext.BenchStreamBackFillFlagName)
	}

	eventsPerPeriod, err := cmd.Flags().GetUint64(cobraext.BenchStreamEventsPerPeriodFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchStreamEventsPerPeriodFlagName)
	}

	if eventsPerPeriod <= 0 {
		return cobraext.FlagParsingError(errors.New("cannot be zero or negative"), cobraext.BenchStreamEventsPerPeriodFlagName)
	}

	periodDuration, err := cmd.Flags().GetDuration(cobraext.BenchStreamPeriodDurationFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchStreamPeriodDurationFlagName)
	}

	if periodDuration < time.Nanosecond {
		return cobraext.FlagParsingError(errors.New("cannot be a negative duration"), cobraext.BenchStreamPeriodDurationFlagName)
	}

	performCleanup, err := cmd.Flags().GetBool(cobraext.BenchStreamPerformCleanupFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchStreamPerformCleanupFlagName)
	}

	timestampField, err := cmd.Flags().GetString(cobraext.BenchStreamTimestampFieldFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchStreamTimestampFieldFlagName)
	}

	packageRootPath, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	repositoryRoot, err := files.FindRepositoryRoot()
	if err != nil {
		return fmt.Errorf("locating repository root failed: %w", err)
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	ctx, stop := signal.Enable(cmd.Context(), logger.Info)
	defer stop()

	esClient, err := stack.NewElasticsearchClientFromProfile(profile)
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch client: %w", err)
	}
	err = esClient.CheckHealth(ctx)
	if err != nil {
		return err
	}

	kc, err := stack.NewKibanaClientFromProfile(profile)
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	withOpts := []stream.OptionFunc{
		stream.WithVariant(variant),
		stream.WithBenchmarkName(benchName),
		stream.WithBackFill(backFill),
		stream.WithEventsPerPeriod(eventsPerPeriod),
		stream.WithPeriodDuration(periodDuration),
		stream.WithPerformCleanup(performCleanup),
		stream.WithTimestampField(timestampField),
		stream.WithPackageRootPath(packageRootPath),
		stream.WithESAPI(esClient.API),
		stream.WithKibanaClient(kc),
		stream.WithProfile(profile),
		stream.WithRepositoryRoot(repositoryRoot),
	}

	runner := stream.NewStreamBenchmark(stream.NewOptions(withOpts...))

	_, err = benchrunner.Run(ctx, runner)
	if err != nil {
		return fmt.Errorf("error running package stream benchmarks: %w", err)
	}

	return nil
}

func getSystemCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Run system benchmarks",
		Long:  "Run system benchmarks for the package",
		Args:  cobra.NoArgs,
		RunE:  systemCommandAction,
	}

	cmd.Flags().StringP(cobraext.BenchPathFlagName, "", "_dev/benchmark/system", cobraext.BenchPathFlagDescription)
	cmd.Flags().StringP(cobraext.BenchNameFlagName, "", "", cobraext.BenchNameFlagDescription)
	cmd.Flags().BoolP(cobraext.BenchReindexToMetricstoreFlagName, "", false, cobraext.BenchReindexToMetricstoreFlagDescription)
	cmd.Flags().DurationP(cobraext.BenchMetricsIntervalFlagName, "", time.Second, cobraext.BenchMetricsIntervalFlagDescription)
	cmd.Flags().DurationP(cobraext.DeferCleanupFlagName, "", 0, cobraext.DeferCleanupFlagDescription)
	cmd.Flags().String(cobraext.VariantFlagName, "", cobraext.VariantFlagDescription)
	cmd.MarkFlagRequired(cobraext.BenchNameFlagName)

	return cmd
}

func systemCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Run system benchmarks for the package")

	variant, err := cmd.Flags().GetString(cobraext.VariantFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.VariantFlagName)
	}

	benchPath, err := cmd.Flags().GetString(cobraext.BenchPathFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BenchPathFlagName)
	}

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

	packageRootPath, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	ctx, stop := signal.Enable(cmd.Context(), logger.Info)
	defer stop()

	esClient, err := stack.NewElasticsearchClientFromProfile(profile)
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch client: %w", err)
	}
	err = esClient.CheckHealth(ctx)
	if err != nil {
		return err
	}

	kc, err := stack.NewKibanaClientFromProfile(profile)
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	withOpts := []system.OptionFunc{
		system.WithVariant(variant),
		system.WithBenchmarkPath(benchPath),
		system.WithBenchmarkName(benchName),
		system.WithDeferCleanup(deferCleanup),
		system.WithMetricsInterval(metricsInterval),
		system.WithDataReindexing(dataReindex),
		system.WithPackageRootPath(packageRootPath),
		system.WithESAPI(esClient.API),
		system.WithKibanaClient(kc),
		system.WithProfile(profile),
	}

	esMetricsClient, err := initializeESMetricsClient(ctx)
	if err != nil {
		return fmt.Errorf("can't create Elasticsearch metrics client: %w", err)
	}
	if esMetricsClient != nil {
		withOpts = append(withOpts, system.WithESMetricsAPI(esMetricsClient.API))
	}

	runner := system.NewSystemBenchmark(system.NewOptions(withOpts...))

	r, err := benchrunner.Run(ctx, runner)
	if err != nil {
		return fmt.Errorf("error running package system benchmarks: %w", err)
	}

	multiReport, ok := r.(reporters.MultiReportable)
	if !ok {
		return fmt.Errorf("system benchmark is expected to return multiple reports")
	}

	reports := multiReport.Split()
	if len(reports) != 2 {
		return fmt.Errorf("system benchmark is expected to return a human an a file report")
	}

	// human report will always be the first
	human := reports[0]
	if err := reporters.WriteReportable(reporters.Output(outputs.ReportOutputSTDOUT), human); err != nil {
		return fmt.Errorf("error writing benchmark report: %w", err)
	}

	// file report will always be the second
	file := reports[1]
	if err := reporters.WriteReportable(reporters.Output(outputs.ReportOutputFile), file); err != nil {
		return fmt.Errorf("error writing benchmark report: %w", err)
	}

	return nil
}

func initializeESMetricsClient(ctx context.Context) (*elasticsearch.Client, error) {
	address := os.Getenv(benchcommon.ESMetricstoreHostEnv)
	apiKey := os.Getenv(benchcommon.ESMetricstoreAPIKeyEnv)
	user := os.Getenv(benchcommon.ESMetricstoreUsernameEnv)
	pass := os.Getenv(benchcommon.ESMetricstorePasswordEnv)
	cacert := os.Getenv(benchcommon.ESMetricstoreCACertificateEnv)
	if address == "" || ((user == "" || pass == "") && apiKey == "") {
		logger.Debugf("can't initialize metricstore, missing environment configuration")
		return nil, nil
	}

	esClient, err := stack.NewElasticsearchClient(
		elasticsearch.OptionWithAddress(address),
		elasticsearch.OptionWithAPIKey(apiKey),
		elasticsearch.OptionWithUsername(user),
		elasticsearch.OptionWithPassword(pass),
		elasticsearch.OptionWithCertificateAuthority(cacert),
	)
	if err != nil {
		return nil, err
	}

	if err := esClient.CheckHealth(ctx); err != nil {
		return nil, err
	}

	return esClient, nil
}
