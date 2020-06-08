package main

import (
	"github.com/elastic/elastic-package/internal/cluster"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func setupClusterCommand() *cobra.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the testing cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	downCommand := &cobra.Command{
		Use:   "down",
		Short: "Take down the testing cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	shellInitCommand := &cobra.Command{
		Use:   "shellinit",
		Short: "Initiate environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			shell, err := cluster.ShellInit()
			if err != nil {
				return errors.Wrap(err, "shellinit failed")
			}
			cmd.Println(shell)
			return nil
		},
	}

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage the testing environment",
		Long:  "Use cluster command to boot up and take down the local testing cluster.",
	}
	cmd.AddCommand(
		upCommand,
		downCommand,
		shellInitCommand)
	return cmd
}
