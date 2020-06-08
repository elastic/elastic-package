package main

import "github.com/spf13/cobra"

func setupClusterCommand() *cobra.Command {
	return &cobra.Command{
		Use: "cluster",
		Short: "Manage the testing environment",
		Long: "Use cluster command to boot up and take down the local testing cluster.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}