// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/stack"
)

var availableServices = map[string]struct{}{
	"elasticsearch":    {},
	"kibana":           {},
	"package-registry": {},
}

const stackLongDescription = `Use this command to spin up a Docker-based Elastic Stack consisting of Elasticsearch, Kibana, and the Package Registry. By default the latest released version of the stack is spun up but it is possible to specify a different version, including SNAPSHOT versions.

For details on how to connect the service with the Elastic stack, see the [HOWTO guide](https://github.com/elastic/elastic-package/blob/master/docs/howto/connect_service_with_elastic_stack.md).`

const stackUpLongDescription = `Use this command to boot up the stack locally.

By default the latest released version of the stack is spun up but it is possible to specify a different version, including SNAPSHOT versions.

To ęxpose local packages in the Package Registry, build them first and boot up the stack from inside of the Git repository containing the package (e.g. elastic/integrations). They will be copied to the development stack (~/.elastic-package/stack/development) and used to build a custom Docker image of the Package Registry.

For details on how to connect the service with the Elastic stack, review the [HOWTO guide](https://github.com/elastic/elastic-package/blob/master/docs/howto/connect_service_with_elastic_stack.md).`

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

			usrProfile, err := profile.LoadProfileFromDefaultLocation(profileName)
			if err == profile.ErrNotAProfile {
				pList, err := availableProfilesAsAList()
				if err != nil {
					return errors.Wrap(err, "error listing known profiles")
				}
				return fmt.Errorf("%s is not a valid profile, known profiles are: %s", profileName, pList)
			}
			if err != nil {
				return errors.Wrap(err, "error loading profile")
			}
			cmd.Printf("Using profile %s.\n", usrProfile.ProfilePath)
			cmd.Println(`Remember to load stack environment variables using 'eval "$(elastic-package stack shellinit)"'.`)

			err = stack.BootUp(stack.Options{
				DaemonMode:   daemonMode,
				StackVersion: stackVersion,
				Services:     services,
				Profile:      usrProfile,
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

			usrProfile, err := profile.LoadProfileFromDefaultLocation(profileName)
			if err == profile.ErrNotAProfile {
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
				Profile: usrProfile,
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

			profile, err := profile.LoadProfileFromDefaultLocation(profileName)
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

			profile, err := profile.LoadProfileFromDefaultLocation(profileName)
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

			target, err := stack.Dump(stack.DumpOptions{
				Output: output,
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

	profileCommand := &cobra.Command{
		Use:   "profiles",
		Short: "Manage stack config profiles",
	}

	// Profile subcommands

	profileNewCommand := &cobra.Command{
		Use:   "new",
		Short: "Create a new profile",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) == 0 {
				return errors.New("new requires an argument")
			}
			newProfileName := args[0]

			fromName, err := cmd.Flags().GetString(cobraext.ProfileFromFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ProfileFromFlagName)
			}

			err = profile.CreateProfileFromDefaultLocation(newProfileName, fromName)
			if err != nil {
				return errors.Wrapf(err, "error creating profile %s from profile %s", newProfileName, fromName)
			}

			fmt.Printf("Created profile %s from %s.\n", newProfileName, fromName)

			return nil
		},
	}
	profileNewCommand.Flags().String(cobraext.ProfileFromFlagName, "default", cobraext.ProfileFromFlagDescription)

	profileDeleteCommand := &cobra.Command{
		Use:   "delete",
		Short: "Delete a profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("delete requires an argument")
			}
			profileName := args[0]

			err := profile.DeleteProfileFromDefaultLocation(profileName)
			if err != nil {
				return errors.Wrap(err, "error deleting profile")
			}

			fmt.Printf("Deleted profile %s\n", profileName)

			return nil
		},
	}

	profileListCommand := &cobra.Command{
		Use:   "list",
		Short: "List available profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := profile.PrintProfilesFromDefaultLocation()
			if err != nil {
				return errors.Wrap(err, "error listing profiles")
			}
			return nil
		},
	}

	profileCommand.AddCommand(profileNewCommand, profileDeleteCommand, profileListCommand)

	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage the Elastic stack",
		Long:  stackLongDescription,
	}
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", lookupEnv(), cobraext.ProfileFlagDescription)
	cmd.AddCommand(
		upCommand,
		downCommand,
		updateCommand,
		shellInitCommand,
		dumpCommand,
		profileCommand)

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func lookupEnv() string {
	env := os.Getenv(cobraext.ProfileNameEnvVar)
	if env == "" {
		return profile.DefaultProfile
	}
	return env

}

func availableProfilesAsAList() ([]string, error) {

	loc, err := locations.NewLocationManager()
	if err != nil {
		return []string{}, errors.Wrap(err, "error fetching profile")
	}

	profileNames := []string{}
	profileList, err := profile.FetchAllProfiles(loc.StackDir())
	if err != nil {
		return profileNames, errors.Wrap(err, "")
	}
	for _, prof := range profileList {
		profileNames = append(profileNames, prof.Name)
	}

	return profileNames, nil
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
