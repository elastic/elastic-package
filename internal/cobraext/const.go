package cobraext

// Flag names and descriptions used by CLI commands.
const (
	DaemonModeFlagName        = "daemon"
	DaemonModeFlagDescription = "daemon mode"

	DatasetFlagName        = "dataset"
	DatasetFlagDescription = "comma-separated datasets to test"

	FailOnMissingFlagName        = "fail-on-missing"
	FailOnMissingFlagDescription = "fail if tests are missing"

	FailFastFlagName        = "fail-fast"
	FailFastFlagDescription = "fail immediately if any file requires updates"

	VerboseFlagName        = "verbose"
	VerboseFlagDescription = "verbose mode"

	StackVersionFlagName    = "version"
	StackVersionDescription = "stack version"
)
