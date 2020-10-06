// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/stack"
)

var availableServices = map[string]struct{}{
	"elasticsearch":    {},
	"kibana":           {},
	"package-registry": {},
}

func setupStackCommand() *cobra.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Boot up the Elastic stack")

			daemonMode, err := cmd.Flags().GetBool(cobraext.DaemonModeFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.DaemonModeFlagName)
			}

			services, err := cmd.Flags().GetStringSlice(cobraext.StackServicesFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackServicesFlagName)
			}

			err = validateServicesFlag(services)
			if err != nil {
				return errors.Wrap(err, "validating services failed")
			}

			stackVersion, err := cmd.Flags().GetString(cobraext.StackVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackVersionFlagName)
			}

			err = stack.BootUp(stack.Options{
				DaemonMode:   daemonMode,
				StackVersion: stackVersion,
				Services:     services,
			})
			if err != nil {
				return errors.Wrap(err, "booting up the stack failed")
			}

			cmd.Println("Done")
			return nil
		},
	}
	upCommand.Flags().BoolP(cobraext.DaemonModeFlagName, "d", false, cobraext.DaemonModeFlagDescription)
	upCommand.Flags().StringSliceP(cobraext.StackServicesFlagName, "s", nil,
		fmt.Sprintf(cobraext.StackServicesFlagDescription, strings.Join(availableServicesAsList(), ", ")))
	upCommand.Flags().StringP(cobraext.StackVersionFlagName, "", stack.DefaultVersion, cobraext.StackVersionDescription)

	downCommand := &cobra.Command{
		Use:   "down",
		Short: "Take down the stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Take down the Elastic stack")

			err := stack.TearDown()
			if err != nil {
				return errors.Wrap(err, "tearing down the stack failed")
			}

			cmd.Println("Done")
			return nil
		},
	}

	updateCommand := &cobra.Command{
		Use:   "update",
		Short: "Update the stack to the most recent versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Update the Elastic stack")

			stackVersion, err := cmd.Flags().GetString(cobraext.StackVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackVersionFlagName)
			}

			err = stack.Update(stack.Options{
				StackVersion: stackVersion,
			})
			if err != nil {
				return errors.Wrap(err, "failed updating the stack images")
			}

			cmd.Println("Done")
			return nil
		},
	}
	updateCommand.Flags().StringP(cobraext.StackVersionFlagName, "", stack.DefaultVersion, cobraext.StackVersionDescription)

	shellInitCommand := &cobra.Command{
		Use:   "shellinit",
		Short: "Export environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			shell, err := stack.ShellInit()
			if err != nil {
				return errors.Wrap(err, "shellinit failed")
			}
			fmt.Println(shell)
			return nil
		},
	}

	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage the Elastic stack",
		Long:  "Use stack command to boot up and take down the local Elastic stack.",
	}
	cmd.AddCommand(
		upCommand,
		downCommand,
		updateCommand,
		shellInitCommand)
	return cmd
}

func availableServicesAsList() []string {
	available := make([]string, len(availableServices))
	i := 0
	for aService := range availableServices {
		available[i] = aService
		i++
	}
	return available
}

func validateServicesFlag(services []string) error {
	selected := map[string]struct{}{}

	for _, aService := range services {
		if _, found := availableServices[aService]; !found {
			return fmt.Errorf("service \"%s\" is not available", aService)
		}

		if _, found := selected[aService]; found {
			return fmt.Errorf("service \"%s\" must be selected at most once", aService)
		}

		selected[aService] = struct{}{}
	}
	return nil
}
