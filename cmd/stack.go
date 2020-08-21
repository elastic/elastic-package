package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/stack"
)

func setupStackCommand() *cobra.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the testing stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			daemonMode, err := cmd.Flags().GetBool(cobraext.DaemonModeFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.DaemonModeFlagName)
			}

			stackVersion, err := cmd.Flags().GetString(cobraext.StackVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackVersionFlagName)
			}

			err = stack.BootUp(daemonMode, stackVersion)
			if err != nil {
				return errors.Wrap(err, "booting up the stack failed")
			}
			return nil
		},
	}
	upCommand.Flags().BoolP(cobraext.DaemonModeFlagName, "d", false, cobraext.DaemonModeFlagDescription)
	upCommand.Flags().StringP(cobraext.StackVersionFlagName, "", stack.DefaultVersion, cobraext.StackVersionDescription)

	downCommand := &cobra.Command{
		Use:   "down",
		Short: "Take down the testing stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := stack.TearDown()
			if err != nil {
				return errors.Wrap(err, "tearing down the stack failed")
			}
			return nil
		},
	}

	updateCommand := &cobra.Command{
		Use:   "update",
		Short: "Updates the stack to the most recent versions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			stackVersion, err := cmd.Flags().GetString(cobraext.StackVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackVersionFlagName)
			}

			err = stack.Update(stackVersion)
			if err != nil {
				return errors.Wrap(err, "failed updating the stack images")
			}
			return nil
		},
	}
	updateCommand.Flags().StringP(cobraext.StackVersionFlagName, "", stack.DefaultVersion, cobraext.StackVersionDescription)

	shellInitCommand := &cobra.Command{
		Use:   "shellinit",
		Short: "Initiate environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			shell, err := stack.ShellInit()
			if err != nil {
				return errors.Wrap(err, "shellinit failed")
			}
			cmd.Println(shell)
			return nil
		},
	}

	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage the testing environment",
		Long:  "Use stack command to boot up and take down the local testing stack.",
	}
	cmd.AddCommand(
		upCommand,
		downCommand,
		updateCommand,
		shellInitCommand)
	return cmd
}
