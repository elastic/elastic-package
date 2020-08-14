package cmd

import (
	"github.com/pkg/errors"
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
	buildInfo, err := version.Info()
	if err != nil {
		return errors.Wrap(err, "reading version information failed")
	}
	cmd.Printf("elastic-package version-hash %s (build time: %s)\n", buildInfo.CommitHash, buildInfo.BuildTime)
	return nil
}
