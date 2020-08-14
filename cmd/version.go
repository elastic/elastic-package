package cmd

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/version"
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
	cmd.Printf("elastic-package version-hash %s (build time: %s)\n", version.CommitHash, version.BuildTimeFormatted())
	return nil
}
