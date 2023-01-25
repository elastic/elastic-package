// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/profile"
)

// jsonFormat is the format for JSON output
const jsonFormat = "json"

// tableFormat is the format for table output
const tableFormat = "table"

// profileNameEnvVar is the name of the environment variable to set the default profile
var profileNameEnvVar = environment.WithElasticPackagePrefix("PROFILE")

func setupProfilesCommand() *cobraext.Command {
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
		Use:   "create",
		Short: "Create a new profile",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) == 0 {
				return fmt.Errorf("create requires an argument")
			}
			newProfileName := args[0]

			fromName, err := cmd.Flags().GetString(cobraext.ProfileFromFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ProfileFromFlagName)
			}
			options := profile.Options{
				Name:        newProfileName,
				FromProfile: fromName,
			}
			err = profile.CreateProfile(options)
			if err != nil {
				return fmt.Errorf("error creating profile %s from profile %s: %s", newProfileName, fromName, err)
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
				return fmt.Errorf("delete requires an argument")
			}
			profileName := args[0]

			err := profile.DeleteProfile(profileName)
			if err != nil {
				return fmt.Errorf("error deleting profile: %s", err)
			}

			fmt.Printf("Deleted profile %s\n", profileName)

			return nil
		},
	}

	profileListCommand := &cobra.Command{
		Use:   "list",
		Short: "List available profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			loc, err := locations.NewLocationManager()
			if err != nil {
				return fmt.Errorf("error fetching profile: %s", err)
			}
			profileList, err := profile.FetchAllProfiles(loc.ProfileDir())
			if err != nil {
				return fmt.Errorf("error listing all profiles: %s", err)
			}

			format, err := cmd.Flags().GetString(cobraext.ProfileFormatFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ProfileFromFlagName)
			}

			switch format {
			case tableFormat:
				return formatTable(profileList)
			case jsonFormat:
				return formatJSON(profileList)
			default:
				return fmt.Errorf("format %s not supported", format)
			}
		},
	}
	profileListCommand.Flags().String(cobraext.ProfileFormatFlagName, tableFormat, cobraext.ProfileFormatFlagDescription)

	profileCommand.AddCommand(profileNewCommand, profileDeleteCommand, profileListCommand)

	return cobraext.NewCommand(profileCommand, cobraext.ContextGlobal)
}

func formatJSON(profileList []profile.Metadata) error {
	data, err := json.Marshal(profileList)
	if err != nil {
		return fmt.Errorf("error listing all profiles in JSON format: %s", err)
	}

	fmt.Print(string(data))

	return nil
}

func formatTable(profileList []profile.Metadata) error {
	table := tablewriter.NewWriter(os.Stdout)
	var profilesTable = profileToList(profileList)

	table.SetHeader([]string{"Name", "Date Created", "User", "Version", "Path"})
	table.SetHeaderColor(
		twColor(tablewriter.Colors{tablewriter.Bold}),
		twColor(tablewriter.Colors{tablewriter.Bold}),
		twColor(tablewriter.Colors{tablewriter.Bold}),
		twColor(tablewriter.Colors{tablewriter.Bold}),
		twColor(tablewriter.Colors{tablewriter.Bold}),
	)
	table.SetColumnColor(
		twColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}),
		tablewriter.Colors{},
		tablewriter.Colors{},
		tablewriter.Colors{},
		tablewriter.Colors{},
	)

	table.SetAutoMergeCells(false)
	table.SetRowLine(true)
	table.AppendBulk(profilesTable)
	table.Render()

	return nil
}

func profileToList(profiles []profile.Metadata) [][]string {
	var profileList [][]string
	for _, profile := range profiles {
		profileList = append(profileList, []string{profile.Name, profile.DateCreated.Format(time.RFC3339), profile.User, profile.Version, profile.Path})
	}

	return profileList
}

func lookupEnv() string {
	env := os.Getenv(profileNameEnvVar)
	if env == "" {
		return profile.DefaultProfile
	}
	return env

}

func availableProfilesAsAList() ([]string, error) {
	loc, err := locations.NewLocationManager()
	if err != nil {
		return []string{}, fmt.Errorf("error fetching profile path: %s", err)
	}

	profileNames := []string{}
	profileList, err := profile.FetchAllProfiles(loc.ProfileDir())
	if err != nil {
		return profileNames, fmt.Errorf("error fetching all profiles: %s", err)
	}
	for _, prof := range profileList {
		profileNames = append(profileNames, prof.Name)
	}

	return profileNames, nil
}
