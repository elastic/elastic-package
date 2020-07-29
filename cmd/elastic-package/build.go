package main

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/builder"
)

func setupBuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the integration",
		Long:  "Use build command to build the integration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := builder.BuildIntegration()
			if err != nil {
				return errors.Wrap(err, "building integration failed")
			}
			return nil
		},
	}
	return cmd
}
