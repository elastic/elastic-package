// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

// Flag names and descriptions used by CLI commands.
const (
	DaemonModeFlagName        = "daemon"
	DaemonModeFlagDescription = "daemon mode"

	DataStreamsFlagName        = "data-streams"
	DataStreamsFlagDescription = "comma-separated data streams to test"

	FailOnMissingFlagName        = "fail-on-missing"
	FailOnMissingFlagDescription = "fail if tests are missing"

	FailFastFlagName        = "fail-fast"
	FailFastFlagDescription = "fail immediately if any file requires updates"

	GenerateTestResultFlagName        = "generate"
	GenerateTestResultFlagDescription = "generate test result file"

	ReportFormatFlagName        = "report-format"
	ReportFormatFlagDescription = "format of test report"

	ReportOutputFlagName        = "report-output"
	ReportOutputFlagDescription = "output location for test report"

	VerboseFlagName        = "verbose"
	VerboseFlagDescription = "verbose mode"

	StackServicesFlagName        = "services"
	StackServicesFlagDescription = "component services (comma-separated values: %s)"

	StackVersionFlagName    = "version"
	StackVersionDescription = "stack version"
)
