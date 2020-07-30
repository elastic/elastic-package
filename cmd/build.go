package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/builder"
)

func setupBuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the package",
		Long:  "Use build command to build the package.",
		RunE:  buildCommandAction,
	}
	return cmd
}

func buildCommandAction(cmd *cobra.Command, args []string) error {
	err := builder.BuildPackage()
	if err != nil {
		return errors.Wrap(err, "building package failed")
	}
	return nil
}
