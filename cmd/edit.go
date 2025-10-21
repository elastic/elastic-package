// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/stack"
)

const editLongDescription = `Use this command to edit assets relevant for the package, e.g. Kibana dashboards.`

const editDashboardsLongDescription = `Use this command to make dashboards editable.

Pass a comma-separated list of dashboard ids with -d or use the interactive prompt to make managed dashboards editable in Kibana.`

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
	editDashboardsCmd.Flags().Bool(cobraext.AllowSnapshotFlagName, false, cobraext.AllowSnapshotDescription)

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
	tlsSkipVerify, err := cmd.Flags().GetBool(cobraext.TLSSkipVerifyFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.TLSSkipVerifyFlagName)
	}
	if tlsSkipVerify {
		opts = append(opts, kibana.TLSSkipVerify())
	}

	allowSnapshot, err := cmd.Flags().GetBool(cobraext.AllowSnapshotFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.AllowSnapshotFlagName)
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	kibanaClient, err := stack.NewKibanaClientFromProfile(profile, opts...)
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	kibanaVersion, err := kibanaClient.Version()
	if err != nil {
		return fmt.Errorf("can't get Kibana status information: %w", err)
	}

	if kibanaVersion.IsSnapshot() {
		message := fmt.Sprintf("editing dashboards from a SNAPSHOT version of Kibana (%s) is discouraged. It could lead to invalid dashboards (for example if they use features that are reverted or modified before the final release)", kibanaVersion.Version())
		if !allowSnapshot {
			return fmt.Errorf("%s. --%s flag can be used to ignore this error", message, cobraext.AllowSnapshotFlagName)
		}
		fmt.Printf("Warning: %s\n", message)
	}

	if len(dashboardIDs) == 0 {
		dashboardIDs, err = promptDashboardIDs(cmd.Context(), kibanaClient)
		if err != nil {
			return fmt.Errorf("prompt for dashboard selection failed: %w", err)
		}

		if len(dashboardIDs) == 0 {
			fmt.Println("No dashboards were found in Kibana.")
			return nil
		}
	}

	updatedDashboardIDs := make([]string, 0, len(dashboardIDs))
	failedDashboardUpdates := make(map[string]error, len(dashboardIDs))
	for _, dashboardID := range dashboardIDs {
		err = kibanaClient.SetManagedSavedObject(cmd.Context(), "dashboard", dashboardID, false)
		if err != nil {
			failedDashboardUpdates[dashboardID] = err
		} else {
			updatedDashboardIDs = append(updatedDashboardIDs, dashboardID)
		}
	}

	if len(updatedDashboardIDs) > 0 {
		urls, err := dashboardURLs(*kibanaClient, updatedDashboardIDs)
		if err != nil {
			cmd.Println(fmt.Sprintf("\nFailed to retrieve dashboard URLS: %s", err.Error()))
			cmd.Println(fmt.Sprintf("The following dashboards are now editable in Kibana:\n%s", strings.Join(updatedDashboardIDs, "\n")))
		} else {
			cmd.Println(fmt.Sprintf("\nThe following dashboards are now editable in Kibana:%s\n\nRemember to export modified dashboards with elastic-package export dashboards", urls))
		}
	}

	if len(failedDashboardUpdates) > 0 {
		var combinedErr error
		for _, err := range failedDashboardUpdates {
			combinedErr = errors.Join(combinedErr, err)
		}
		fmt.Println("")
		return fmt.Errorf("failed to make one or more dashboards editable: %s", combinedErr.Error())
	}

	fmt.Println("\nDone")
	return nil
}

func dashboardURLs(kibanaClient kibana.Client, dashboardIDs []string) (string, error) {
	kibanaHost := kibanaClient.Address()
	kibanaURL, err := url.Parse(kibanaHost)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve Kibana URL: %w", err)
	}
	var urls strings.Builder
	for _, dashboardID := range dashboardIDs {
		dashboardURL := *kibanaURL
		dashboardURL.Path = "app/dashboards"
		dashboardURL.Fragment = "/view/" + dashboardID
		fmt.Fprintf(&urls, "\n%s", dashboardURL.String())
	}
	return urls.String(), nil
}
