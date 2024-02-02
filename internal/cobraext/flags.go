// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

// Global flags
const (
	VerboseFlagName        = "verbose"
	VerboseFlagDescription = "verbose mode"
)

// Primary flags reused by multiple commands
const (
	PackageRootFlagName        = "root"
	PackageRootFlagShorthand   = "R"
	PackageRootFlagDescription = "root directory of the package"

	PackageFlagName        = "package"
	PackageFlagShorthand   = "P"
	PackageFlagDescription = "name of the package"
)

// Flag names and descriptions used by CLI commands
const (
	AgentPolicyFlagName    = "agent-policy"
	AgentPolicyDescription = "name of the agent policy"

	AllowSnapshotFlagName    = "allow-snapshot"
	AllowSnapshotDescription = "allow to export dashboards from a Elastic stack SNAPSHOT version"

	BenchPathFlagName        = "path"
	BenchPathFlagDescription = "path of the benchmark scenario to run"

	BenchNameFlagName        = "benchmark"
	BenchNameFlagDescription = "name of the benchmark scenario to run"

	BenchNumTopProcsFlagName        = "num-top-procs"
	BenchNumTopProcsFlagDescription = "number of top processors to show in the benchmarks results"

	BenchMetricsIntervalFlagName        = "metrics-collection-interval"
	BenchMetricsIntervalFlagDescription = "the interval at which metrics are collected"

	BenchReindexToMetricstoreFlagName        = "reindex-to-metricstore"
	BenchReindexToMetricstoreFlagDescription = "if set the documents from the benchmark will be reindexed to the metricstore for posterior analysis"

	BenchReportNewPathFlagName        = "new"
	BenchReportNewPathFlagDescription = "path of the directory containing the new benchmarks of the report"

	BenchReportOldPathFlagName        = "old"
	BenchReportOldPathFlagDescription = "path of the directory containing the old benchmarks to compare against to generate the report"

	BenchThresholdFlagName        = "threshold"
	BenchThresholdFlagDescription = "threshold to assume a benchmark report has significantly changed"

	BenchWithTestSamplesFlagName        = "use-test-samples"
	BenchWithTestSamplesFlagDescription = "use test samples for the benchmarks"

	BenchCorpusRallyTrackOutputDirFlagName        = "rally-track-output-dir"
	BenchCorpusRallyTrackOutputDirFlagDescription = "output dir of the rally track: if present the command will save the generated rally track"

	BenchCorpusRallyDryRunFlagName        = "dry-run"
	BenchCorpusRallyDryRunFlagDescription = "do not run rally but just generate the rally track"

	BenchCorpusRallyPackageFromRegistryFlagName        = "package-from-registry"
	BenchCorpusRallyPackageFromRegistryFlagDescription = "fetch package from registry instead of local directory, expected format: <package>-<version>"

	BenchCorpusRallyUseCorpusAtPathFlagName        = "use-corpus-at-path"
	BenchCorpusRallyUseCorpusAtPathFlagDescription = "path of the corpus to use for the benchmark: if present no new corpus will be generated"

	BenchStreamBackFillFlagName        = "backfill"
	BenchStreamBackFillFlagDescription = "amount of time to ingest events for, starting before now: expressed as a positive duration"

	BenchStreamEventsPerPeriodFlagName        = "events-per-period"
	BenchStreamEventsPerPeriodFlagDescription = "number of events to ingest at every ingestion cycle"

	BenchStreamPeriodDurationFlagName        = "period-duration"
	BenchStreamPeriodDurationFlagDescription = "duration of the period between each ingestion cycle: expressed as a positive duration"

	BenchStreamPerformCleanupFlagName        = "perform-cleanup"
	BenchStreamPerformCleanupFlagDescription = "whether to perform cleanup at the beginning and after finishing streaming: default to false, if provided will delete data before and after streaming events and uninstall the package at the end"

	BenchStreamTimestampFieldFlagName        = "timestamp-field"
	BenchStreamTimestampFieldFlagDescription = "name of the field that's used in the generator config as `@timestamp`"

	BuildSkipValidationFlagName        = "skip-validation"
	BuildSkipValidationFlagDescription = "skip validation of the built package, use only if all validation issues have been acknowledged"

	BuildZipFlagName        = "zip"
	BuildZipFlagDescription = "archive the built package"

	ChangelogAddNextFlagName        = "next"
	ChangelogAddNextFlagDescription = "changelog entry is added in the next `major`, `minor` or `patch` version"

	ChangelogAddVersionFlagName        = "version"
	ChangelogAddVersionFlagDescription = "changelog entry is added in the given version"

	ChangelogAddDescriptionFlagName        = "description"
	ChangelogAddDescriptionFlagDescription = "description for the changelog entry"

	ChangelogAddTypeFlagName        = "type"
	ChangelogAddTypeFlagDescription = "type of change (bugfix, enhancement or breaking-change) for the changelog entry"

	ChangelogAddLinkFlagName        = "link"
	ChangelogAddLinkFlagDescription = "link to the pull request or issue with more information about the changelog entry"

	CheckConditionFlagName        = "check-condition"
	CheckConditionFlagDescription = "check if the condition is met for the package, but don't install the package (e.g. kibana.version=7.10.0)"

	DaemonModeFlagName        = "daemon"
	DaemonModeFlagDescription = "daemon mode"

	DashboardIDsFlagName        = "id"
	DashboardIDsFlagDescription = "Kibana dashboard IDs (comma-separated values)"

	DataStreamFlagName        = "data-stream"
	DataStreamFlagDescription = "use service stack related to the data stream"

	DataStreamsFlagName        = "data-streams"
	DataStreamsFlagDescription = "comma-separated data streams to test"

	DeferCleanupFlagName        = "defer-cleanup"
	DeferCleanupFlagDescription = "defer test cleanup for debugging purposes"

	DumpOutputFlagName        = "output"
	DumpOutputFlagDescription = "path to directory where exported assets will be stored"

	FailOnMissingFlagName        = "fail-on-missing"
	FailOnMissingFlagDescription = "fail if tests are missing"

	FailFastFlagName                  = "fail-fast"
	FailFastFlagDescription           = "fail immediately if any file requires updates (do not overwrite)"
	GenerateTestResultFlagName        = "generate"
	GenerateTestResultFlagDescription = "generate test result file"

	ProfileFlagName        = "profile"
	ProfileFlagDescription = "select a profile to use for the stack configuration. Can also be set with %s"

	ProfileFromFlagName        = "from"
	ProfileFromFlagDescription = "copy profile from the specified existing profile"

	ProfileFormatFlagName        = "format"
	ProfileFormatFlagDescription = "format of the profiles list (table | json)"

	ReportFormatFlagName        = "report-format"
	ReportFormatFlagDescription = "format of test report"

	ReportFullFlagName        = "full"
	ReportFullFlagDescription = "whether to show the full report or a summary"

	ReportOutputFlagName        = "report-output"
	ReportOutputFlagDescription = "output type for test report, eg: stdout, file"

	ReportOutputPathFlagName        = "report-output-path"
	ReportOutputPathFlagDescription = "output path for test report (defaults to %q in build directory)"

	ShowAllFlagName        = "all"
	ShowAllFlagDescription = "show all deployed package revisions"

	ShellInitShellFlagName    = "shell"
	ShellInitShellDescription = "change output shell code compatibility. Use 'detect' to use integrated shell detection; suggested to not change unless detection is not working"
	ShellInitShellDetect      = "auto"

	SignPackageFlagName        = "sign"
	SignPackageFlagDescription = "sign package"

	TLSSkipVerifyFlagName        = "tls-skip-verify"
	TLSSkipVerifyFlagDescription = "skip TLS verify"

	StackProviderFlagName        = "provider"
	StackProviderFlagDescription = "service provider to start a stack (%s)"

	StackServicesFlagName        = "services"
	StackServicesFlagDescription = "component services (comma-separated values: \"%s\")"

	StackVersionFlagName        = "version"
	StackVersionFlagDescription = "stack version"

	StackDumpOutputFlagName        = "output"
	StackDumpOutputFlagDescription = "output location for the stack dump"

	StackUserParameterFlagName      = "parameter"
	StackUserParameterFlagShorthand = "U"
	StackUserParameterDescription   = "optional parameter for the stack provider, as key=value"

	StatusKibanaVersionFlagName        = "kibana-version"
	StatusKibanaVersionFlagDescription = "show packages for the given kibana version"

	StatusExtraInfoFlagName        = "info"
	StatusExtraInfoFlagDescription = "show additional information (comma-separated values: \"%s\")"

	StatusFormatFlagName        = "format"
	StatusFormatFlagDescription = "output format (\"%s\")"

	TestCoverageFlagName        = "test-coverage"
	TestCoverageFlagDescription = "enable test coverage reports"

	TestCoverageFormatFlagName        = "coverage-format"
	TestCoverageFormatFlagDescription = "set format for coverage reports: %s"

	VariantFlagName        = "variant"
	VariantFlagDescription = "service variant"

	ZipPackageFilePathFlagName        = "zip"
	ZipPackageFilePathFlagShorthand   = "z"
	ZipPackageFilePathFlagDescription = "path to the zip package file (*.zip)"

	// To be removed promote commands flags
	DirectionFlagName        = "direction"
	DirectionFlagDescription = "promotion direction"

	NewestOnlyFlagName        = "newest-only"
	NewestOnlyFlagDescription = "promote newest packages and remove old ones"

	PromotedPackagesFlagName        = "packages"
	PromotedPackagesFlagDescription = "packages to be promoted (comma-separated values: apache-1.2.3,nginx-5.6.7)"

	// To be removed publish commands flags
	ForkFlagName        = "fork"
	ForkFlagDescription = "use fork mode (set to \"false\" if user can't fork the storage repository)"

	SkipPullRequestFlagName        = "skip-pull-request"
	SkipPullRequestFlagDescription = "skip opening a new pull request"
)
