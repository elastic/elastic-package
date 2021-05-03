// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/profile"
)

func setupProfilesCommand() *cobraext.Command {

	// Profile subcommands

	profilesLongDescription := `Use this command to add, remove, and manage multiple config profiles.
	
Individual user profiles appear in ~/.elastic-package/stack, and contain all the config files needed by the "stack" subcommand. 
Once a new profile is created, it can be specified with the -p flag, or the ELASTIC_PACKAGE_PROFILE environment variable.
User profiles are not overwritten on upgrade of elastic-stack, and can be freely modified to allow for different stack configs.`

	profileCommand := &cobra.Command{
		Use:   "profiles",
		Short: "Manage stack config profiles",
		Long:  profilesLongDescription,
	}

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

	return cobraext.NewCommand(profileCommand, cobraext.ContextGlobal)

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
