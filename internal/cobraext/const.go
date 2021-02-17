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

	DataStreamsFlagName        = "data-streams"
	DataStreamsFlagDescription = "comma-separated data streams to test"

	DeferCleanupFlagName        = "defer-cleanup"
	DeferCleanupFlagDescription = "defer test cleanup for debugging purposes"

	FailOnMissingFlagName        = "fail-on-missing"
	FailOnMissingFlagDescription = "fail if tests are missing"

	FailFastFlagName        = "fail-fast"
	FailFastFlagDescription = "fail immediately if any file requires updates (do not overwrite)"

	GenerateTestResultFlagName        = "generate"
	GenerateTestResultFlagDescription = "generate test result file"

	ReportFormatFlagName        = "report-format"
	ReportFormatFlagDescription = "format of test report"

	ReportOutputFlagName        = "report-output"
	ReportOutputFlagDescription = "output location for test report"

	VerboseFlagName        = "verbose"
	VerboseFlagDescription = "verbose mode"

	StackServicesFlagName        = "services"
	StackServicesFlagDescription = "component services (comma-separated values: \"%s\")"

	StackVersionFlagName        = "version"
	StackVersionFlagDescription = "stack version"

	StackDumpOutputFlagName        = "output"
	StackDumpOutputFlagDescription = "output location for the stack dump"
)
