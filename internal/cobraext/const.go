// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

// Flag names and descriptions used by CLI commands.
const (
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

	NewestOnlyFlagName        = "newest-only"
	NewestOnlyFlagDescription = "promote newest packages and remove old ones"

	PackagesFlagName        = "packages"
	PackagesFlagDescription = "packages to be promoted (comma-separated values: apache-1.2.3,nginx-5.6.7)"

	ReportFormatFlagName        = "report-format"
	ReportFormatFlagDescription = "format of test report"

	ReportOutputFlagName        = "report-output"
	ReportOutputFlagDescription = "output location for test report"

	ShowAllFlagName        = "all"
	ShowAllFlagDescription = "show all deployed package revisions"

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

	TestCoverageFlagName        = "test-coverage"
	TestCoverageFlagDescription = "generate Cobertura test coverage reports"

	VariantFlagName        = "variant"
	VariantFlagDescription = "service variant"

	VerboseFlagName        = "verbose"
	VerboseFlagDescription = "verbose mode"
)
