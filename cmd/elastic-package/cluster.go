package main

import "github.com/spf13/cobra"

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

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage the testing environment",
		Long:  "Use cluster command to boot up and take down the local testing cluster.",
	}
	cmd.AddCommand(
		upCommand,
		downCommand)
	return cmd
}
