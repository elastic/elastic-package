// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/stack"
)

const editLongDescription = `Use this command to edit assets relevant for the package, e.g. Kibana dashboards.`

const editDashboardsLongDescription = `Use this command to make dashboards editable.

This command re-imports the selected dashboards from Kibana after making them editable.`

func setupEditCommand() *cobraext.Command {
	editDashboardsCmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Make dashboards editable in Kibana",
		Long:  editDashboardsLongDescription,
		Args:  cobra.NoArgs,
		RunE:  editDashboardsCmd,
	}
	editDashboardsCmd.Flags().StringSliceP(cobraext.DashboardIDsFlagName, "d", nil, cobraext.DashboardIDsFlagDescription)
	editDashboardsCmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit package assets",
		Long:  editLongDescription,
	}
	cmd.AddCommand(editDashboardsCmd)
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func editDashboardsCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("Make Kibana dashboards editable")

	dashboardIDs, err := cmd.Flags().GetStringSlice(cobraext.DashboardIDsFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.DashboardIDsFlagName)
	}

	common.TrimStringSlice(dashboardIDs)

	var opts []kibana.ClientOption
	tlsSkipVerify, _ := cmd.Flags().GetBool(cobraext.TLSSkipVerifyFlagName)
	if tlsSkipVerify {
		opts = append(opts, kibana.TLSSkipVerify())
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	kibanaClient, err := stack.NewKibanaClientFromProfile(profile, opts...)
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	if len(dashboardIDs) == 0 {
		dashboardIDs, err = promptDashboardIDs(kibanaClient)
		if err != nil {
			return fmt.Errorf("prompt for dashboard selection failed: %w", err)
		}

		if len(dashboardIDs) == 0 {
			fmt.Println("No dashboards were found in Kibana.")
			return nil
		}
	}

	for _, dashboardID := range dashboardIDs {
		err = kibanaClient.SetManagedSavedObject("dashboard", dashboardID, false)
		if err != nil {
			return fmt.Errorf("failed to make dashboards editable: %w", err)
		}
	}

	cmd.Println("Done")
	return nil
}
