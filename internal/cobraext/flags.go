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

	BenchNumTopProcsFlagName        = "num-top-procs"
	BenchNumTopProcsFlagDescription = "number of top processors to show in the benchmarks results"

	BenchWithTestSamplesFlagName        = "use-test-samples"
	BenchWithTestSamplesFlagDescription = "use test samples for the benchmarks"

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

	DirectionFlagName        = "direction"
	DirectionFlagDescription = "promotion direction"

	DeferCleanupFlagName        = "defer-cleanup"
	DeferCleanupFlagDescription = "defer test cleanup for debugging purposes"

	DumpOutputFlagName        = "output"
	DumpOutputFlagDescription = "path to directory where exported assets will be stored"

	FailOnMissingFlagName        = "fail-on-missing"
	FailOnMissingFlagDescription = "fail if tests are missing"

	FailFastFlagName        = "fail-fast"
	FailFastFlagDescription = "fail immediately if any file requires updates (do not overwrite)"

	ForkFlagName        = "fork"
	ForkFlagDescription = "use fork mode (set to \"false\" if user can't fork the storage repository)"

	GenerateTestResultFlagName        = "generate"
	GenerateTestResultFlagDescription = "generate test result file"

	ProfileFlagName        = "profile"
	ProfileFlagDescription = "select a profile to use for the stack configuration. Can also be set with %s"

	ProfileFromFlagName        = "from"
	ProfileFromFlagDescription = "copy profile from the specified existing profile"

	ProfileFormatFlagName        = "format"
	ProfileFormatFlagDescription = "format of the profiles list (table | json)"

	NewestOnlyFlagName        = "newest-only"
	NewestOnlyFlagDescription = "promote newest packages and remove old ones"

	PromotedPackagesFlagName        = "packages"
	PromotedPackagesFlagDescription = "packages to be promoted (comma-separated values: apache-1.2.3,nginx-5.6.7)"

	ReportFormatFlagName        = "report-format"
	ReportFormatFlagDescription = "format of test report"

	ReportOutputFlagName        = "report-output"
	ReportOutputFlagDescription = "output location for test report"

	ShowAllFlagName        = "all"
	ShowAllFlagDescription = "show all deployed package revisions"

	SignPackageFlagName        = "sign"
	SignPackageFlagDescription = "sign package"

	SkipPullRequestFlagName        = "skip-pull-request"
	SkipPullRequestFlagDescription = "skip opening a new pull request"

	TLSSkipVerifyFlagName        = "tls-skip-verify"
	TLSSkipVerifyFlagDescription = "skip TLS verify"

	StackServicesFlagName        = "services"
	StackServicesFlagDescription = "component services (comma-separated values: \"%s\")"

	StackVersionFlagName        = "version"
	StackVersionFlagDescription = "stack version"

	StackDumpOutputFlagName        = "output"
	StackDumpOutputFlagDescription = "output location for the stack dump"

	StatusKibanaVersionFlagName        = "kibana-version"
	StatusKibanaVersionFlagDescription = "show packages for the given kibana version"

	TestCoverageFlagName        = "test-coverage"
	TestCoverageFlagDescription = "generate Cobertura test coverage reports"

	VariantFlagName        = "variant"
	VariantFlagDescription = "service variant"
)
