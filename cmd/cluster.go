package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cluster"
)

func setupClusterCommand() *cobra.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the testing cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := cmd.Flags().GetBool("d")
			if err != nil {
				return err
			}
			err = cluster.BootUp(d)
			if err != nil {
				return errors.Wrap(err, "booting up the cluster failed")
			}
			return nil
		},
	}
	upCommand.Flags().BoolP("daemon","d", false, "Run cluster as daemon")

	downCommand := &cobra.Command{
		Use:   "down",
		Short: "Take down the testing cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cluster.TearDown()
			if err != nil {
				return errors.Wrap(err, "tearing down the cluster failed")
			}
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
