package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// BuildTime is the build time of the binary (set externally with ldflags).
	BuildTime = "unknown"

	// CommitHash is the Git hash of the branch, used for version purposes (set externally with ldflags).
	CommitHash = "undefined"
)

func setupVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Long:  "Use version command to show the application version.",
		RunE:  versionCommandAction,
	}
	return cmd
}

func versionCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Printf("elastic-package version-hash %s (build time: %s)\n", CommitHash, BuildTime)
	return nil
}
