// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/olekukonko/tablewriter"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/telemetry"
)

// jsonFormat is the format for JSON output
const jsonFormat = "json"

// tableFormat is the format for table output
const tableFormat = "table"

func setupProfilesCommand() *cobraext.Command {
	profilesLongDescription := `Use this command to add, remove, and manage multiple config profiles.

Individual user profiles appear in ~/.elastic-package/stack, and contain all the config files needed by the "stack" subcommand.
Once a new profile is created, it can be specified with the -p flag, or the ELASTIC_PACKAGE_PROFILE environment variable.
User profiles can be configured with a "config.yml" file in the profile directory.`

	profileCommand := &cobra.Command{
		Use:   "profiles",
		Short: "Manage stack config profiles",
		Long:  profilesLongDescription,
	}

	profileNewCommand := &cobra.Command{
		Use:   "create [profile]",
		Short: "Create a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
				return fmt.Errorf("error creating profile %s from profile %s: %w", newProfileName, fromName, err)
			}

			if fromName == "" {
				fmt.Printf("Created profile %q.\n", newProfileName)
			} else {
				fmt.Printf("Created profile %q from %q.\n", newProfileName, fromName)
			}

			return nil
		},
	}
	profileNewCommand.Flags().String(cobraext.ProfileFromFlagName, "", cobraext.ProfileFromFlagDescription)

	profileDeleteCommand := &cobra.Command{
		Use:   "delete [profile]",
		Short: "Delete a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			config, err := install.Configuration()
			if err != nil {
				return fmt.Errorf("failed to load current configuration: %w", err)
			}

			err = profile.DeleteProfile(profileName)
			if err != nil {
				return fmt.Errorf("error deleting profile: %w", err)
			}

			if currentProfile := config.CurrentProfile(); currentProfile == profileName {
				config.SetCurrentProfile(profile.DefaultProfile)

				location, err := locations.NewLocationManager()
				if err != nil {
					return fmt.Errorf("error fetching profile: %w", err)
				}
				err = install.WriteConfigFile(location, config)
				if err != nil {
					return fmt.Errorf("failed to store configuration: %w", err)
				}

				cmd.Printf("%q was the current profile. Default profile will be used now.\n", profileName)
			}

			fmt.Printf("Deleted profile %q.\n", profileName)

			return nil
		},
	}

	profileListCommand := &cobra.Command{
		Use:   "list",
		Short: "List available profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			globalCtx, span := telemetry.StartSpanForCommand(telemetry.CmdTracer, cmd)
			defer span.End()

			loc, err := locations.NewLocationManager()
			if err != nil {
				telemetry.ProfilesListFailureCnt.Add(globalCtx, 1)
				return fmt.Errorf("error fetching profile: %w", err)
			}

			_, fetchSpan := telemetry.CmdTracer.Start(globalCtx, "Fetch all profiles")
			profileList, err := profile.FetchAllProfiles(loc.ProfileDir())
			if err != nil {
				telemetry.ProfilesListFailureCnt.Add(globalCtx, 1)
				return fmt.Errorf("error listing all profiles: %w", err)
			}
			if len(profileList) == 0 {
				fmt.Println("There are no profiles yet.")
				telemetry.ProfilesListSuccessCnt.Add(globalCtx, 1)
				return nil
			}
			fetchSpan.End()

			format, err := cmd.Flags().GetString(cobraext.ProfileFormatFlagName)
			if err != nil {
				telemetry.ProfilesListFailureCnt.Add(globalCtx, 1)
				return cobraext.FlagParsingError(err, cobraext.ProfileFormatFlagName)
			}

			_, formatSpan := telemetry.CmdTracer.Start(globalCtx, "Format profiles",
				trace.WithAttributes(
					attribute.String("elastic-package.profiles.format", format),
				),
			)
			switch format {
			case tableFormat:
				var config *install.ApplicationConfiguration
				config, err = install.Configuration()
				if err != nil {
					telemetry.ProfilesListFailureCnt.Add(globalCtx, 1)
					return fmt.Errorf("failed to load current configuration: %w", err)
				}
				err = formatTable(loc.ProfileDir(), profileList, config.CurrentProfile())
			case jsonFormat:
				err = formatJSON(profileList)
			default:
				err = fmt.Errorf("format %s not supported", format)
			}
			if err != nil {
				formatSpan.RecordError(err)
				formatSpan.SetStatus(codes.Error, "error formatting profiles")
				telemetry.ProfilesListFailureCnt.Add(globalCtx, 1)
			} else {
				formatSpan.SetStatus(codes.Ok, "profiles listed")
				telemetry.ProfilesListSuccessCnt.Add(globalCtx, 1)
			}
			formatSpan.End()
			return err
		},
	}
	profileListCommand.Flags().String(cobraext.ProfileFormatFlagName, tableFormat, cobraext.ProfileFormatFlagDescription)

	profileUseCommand := &cobra.Command{
		Use:   "use [profile]",
		Short: "Sets the profile to use when no other is specified",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			_, err := profile.LoadProfile(profileName)
			if err != nil {
				return fmt.Errorf("cannot use profile %q: %v", profileName, err)
			}

			location, err := locations.NewLocationManager()
			if err != nil {
				return fmt.Errorf("error fetching profile: %w", err)
			}

			config, err := install.Configuration()
			if err != nil {
				return fmt.Errorf("failed to load current configuration: %w", err)
			}
			config.SetCurrentProfile(profileName)

			err = install.WriteConfigFile(location, config)
			if err != nil {
				return fmt.Errorf("failed to store configuration: %w", err)
			}

			cmd.Printf("Current profile set to %q.\n", profileName)
			return nil
		},
	}

	profileCommand.AddCommand(
		profileNewCommand,
		profileDeleteCommand,
		profileListCommand,
		profileUseCommand,
	)

	return cobraext.NewCommand(profileCommand, cobraext.ContextGlobal)
}

func formatJSON(profileList []profile.Metadata) error {
	data, err := json.Marshal(profileList)
	if err != nil {
		return fmt.Errorf("error listing all profiles in JSON format: %w", err)
	}

	fmt.Print(string(data))

	return nil
}

func formatTable(profilesDir string, profileList []profile.Metadata, currentProfile string) error {
	table := tablewriter.NewWriter(os.Stdout)
	profilesTable := profileToList(profilesDir, profileList, currentProfile)

	table.SetHeader([]string{"Name", "Date Created", "Version", "Path"})
	table.SetHeaderColor(
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
	)

	table.SetAutoMergeCells(false)
	table.SetRowLine(true)
	table.AppendBulk(profilesTable)
	table.Render()

	return nil
}

func profileToList(profilesDir string, profiles []profile.Metadata, currentProfile string) [][]string {
	var profileList [][]string
	for _, profile := range profiles {
		name := profile.Name
		if name == currentProfile {
			name = name + " (current)"
		}
		profilePath := filepath.Join(profilesDir, profile.Name)
		dateCreated := profile.DateCreated.Format(time.RFC822)
		profileList = append(profileList, []string{name, dateCreated, profile.Version, profilePath})
	}

	return profileList
}
