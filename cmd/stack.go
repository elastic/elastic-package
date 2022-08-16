// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

var availableServices = map[string]struct{}{
	"elastic-agent":    {},
	"elasticsearch":    {},
	"fleet-server":     {},
	"kibana":           {},
	"package-registry": {},
}

const stackLongDescription = `Use this command to spin up a Docker-based Elastic Stack consisting of Elasticsearch, Kibana, and the Package Registry. By default the latest released version of the stack is spun up but it is possible to specify a different version, including SNAPSHOT versions by appending --version <version>.

Be aware that a common issue while trying to boot up the stack is that your Docker environments settings are too low in terms of memory threshold.

For details on how to connect the service with the Elastic stack, see the [service command](https://github.com/elastic/elastic-package/blob/main/README.md#elastic-package-service).`

const stackUpLongDescription = `Use this command to boot up the stack locally.

By default the latest released version of the stack is spun up but it is possible to specify a different version, including SNAPSHOT versions by appending --version <version>.

Be aware that a common issue while trying to boot up the stack is that your Docker environments settings are too low in terms of memory threshold.

To ęxpose local packages in the Package Registry, build them first and boot up the stack from inside of the Git repository containing the package (e.g. elastic/integrations). They will be copied to the development stack (~/.elastic-package/stack/development) and used to build a custom Docker image of the Package Registry.

For details on how to connect the service with the Elastic stack, see the [service command](https://github.com/elastic/elastic-package/blob/main/README.md#elastic-package-service).`

func setupStackCommand() *cobraext.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the stack",
		Long:  stackUpLongDescription,
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

			common.TrimStringSlice(services)

			err = validateServicesFlag(services)
			if err != nil {
				return errors.Wrap(err, "validating services failed")
			}

			stackVersion, err := cmd.Flags().GetString(cobraext.StackVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackVersionFlagName)
			}

			profileName, err := cmd.Flags().GetString(cobraext.ProfileFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ProfileFlagName)
			}

			userProfile, err := profile.LoadProfile(profileName)
			if errors.Is(err, profile.ErrNotAProfile) {
				pList, err := availableProfilesAsAList()
				if err != nil {
					return errors.Wrap(err, "error listing known profiles")
				}
				return fmt.Errorf("%s is not a valid profile, known profiles are: %s", profileName, pList)
			}
			if err != nil {
				return errors.Wrap(err, "error loading profile")
			}

			// Print information before starting the stack, for cases where
			// this is executed in the foreground, without daemon mode.
			cmd.Printf("Using profile %s.\n", userProfile.ProfilePath)
			cmd.Println(`Remember to load stack environment variables using 'eval "$(elastic-package stack shellinit)"'.`)
			err = printInitConfig(cmd, userProfile)
			if err != nil {
				return err
			}

			err = stack.BootUp(stack.Options{
				DaemonMode:   daemonMode,
				StackVersion: stackVersion,
				Services:     services,
				Profile:      userProfile,
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
		fmt.Sprintf(cobraext.StackServicesFlagDescription, strings.Join(availableServicesAsList(), ",")))
	upCommand.Flags().StringP(cobraext.StackVersionFlagName, "", install.DefaultStackVersion, cobraext.StackVersionFlagDescription)

	downCommand := &cobra.Command{
		Use:   "down",
		Short: "Take down the stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Take down the Elastic stack")

			profileName, err := cmd.Flags().GetString(cobraext.ProfileFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ProfileFlagName)
			}

			userProfile, err := profile.LoadProfile(profileName)
			if errors.Is(err, profile.ErrNotAProfile) {
				pList, err := availableProfilesAsAList()
				if err != nil {
					return errors.Wrap(err, "error listing known profiles")
				}
				return fmt.Errorf("%s is not a valid profile, known profiles are: %s", profileName, pList)
			}

			if err != nil {
				return errors.Wrap(err, "error loading profile")
			}

			err = stack.TearDown(stack.Options{
				Profile: userProfile,
			})
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

			profileName, err := cmd.Flags().GetString(cobraext.ProfileFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ProfileFlagName)
			}

			profile, err := profile.LoadProfile(profileName)
			if err != nil {
				return errors.Wrap(err, "error loading profile")
			}

			stackVersion, err := cmd.Flags().GetString(cobraext.StackVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackVersionFlagName)
			}

			err = stack.Update(stack.Options{
				StackVersion: stackVersion,
				Profile:      profile,
			})
			if err != nil {
				return errors.Wrap(err, "failed updating the stack images")
			}

			cmd.Println("Done")
			return nil
		},
	}
	updateCommand.Flags().StringP(cobraext.StackVersionFlagName, "", install.DefaultStackVersion, cobraext.StackVersionFlagDescription)

	shellInitCommand := &cobra.Command{
		Use:   "shellinit",
		Short: "Export environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := cmd.Flags().GetString(cobraext.ProfileFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ProfileFlagName)
			}

			profile, err := profile.LoadProfile(profileName)
			if err != nil {
				return errors.Wrap(err, "error loading profile")
			}

			shell, err := stack.ShellInit(profile)
			if err != nil {
				return errors.Wrap(err, "shellinit failed")
			}
			fmt.Println(shell)
			return nil
		},
	}

	dumpCommand := &cobra.Command{
		Use:   "dump",
		Short: "Dump stack data for debug purposes",
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := cmd.Flags().GetString(cobraext.StackDumpOutputFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackDumpOutputFlagName)
			}

			profileName, err := cmd.Flags().GetString(cobraext.ProfileFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ProfileFlagName)
			}

			profile, err := profile.LoadProfile(profileName)
			if err != nil {
				return errors.Wrap(err, "error loading profile")
			}

			target, err := stack.Dump(stack.DumpOptions{
				Output:  output,
				Profile: profile,
			})
			if err != nil {
				return errors.Wrap(err, "dump failed")
			}

			cmd.Printf("Path to stack dump: %s\n", target)

			cmd.Println("Done")
			return nil
		},
	}
	dumpCommand.Flags().StringP(cobraext.StackDumpOutputFlagName, "", "elastic-stack-dump", cobraext.StackDumpOutputFlagDescription)

	statusCommand := &cobra.Command{
		Use:   "status",
		Short: "Show status of the stack services",
		RunE: func(cmd *cobra.Command, args []string) error {
			servicesStatus, err := stack.Status()
			if err != nil {
				return errors.Wrap(err, "failed getting stack status")
			}

			cmd.Println("Status of Elastic stack services:")
			printStatus(cmd, servicesStatus)
			return nil
		},
	}

	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage the Elastic stack",
		Long:  stackLongDescription,
	}
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", lookupEnv(), fmt.Sprintf(cobraext.ProfileFlagDescription, profileNameEnvVar))
	cmd.AddCommand(
		upCommand,
		downCommand,
		updateCommand,
		shellInitCommand,
		dumpCommand,
		statusCommand)

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
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

func printInitConfig(cmd *cobra.Command, profile *profile.Profile) error {
	initConfig, err := stack.StackInitConfig(profile)
	if err != nil {
		return nil
	}
	cmd.Printf("Elasticsearch host: %s\n", initConfig.ElasticsearchHostPort)
	cmd.Printf("Kibana host: %s\n", initConfig.KibanaHostPort)
	cmd.Printf("Username: %s\n", initConfig.ElasticsearchUsername)
	cmd.Printf("Password: %s\n", initConfig.ElasticsearchPassword)
	return nil
}

func printStatus(cmd *cobra.Command, servicesStatus []stack.ServiceStatus) {
	if len(servicesStatus) == 0 {
		cmd.Printf(" - No service running\n")
		return
	}
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Service", "Version", "Status"})

	for _, service := range servicesStatus {
		t.AppendRow(table.Row{service.Name, service.Version, service.Status})
	}
	t.SetStyle(table.StyleRounded)
	cmd.Println(t.Render())
}
