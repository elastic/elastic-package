// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/kibana"
)

const exportLongDescription = `Use this command to export assets relevant for the package, e.g. Kibana dashboards.`

const exportDashboardsLongDescription = `Use this command to export dashboards with referenced objects from the Kibana instance.

Use this command to download selected dashboards and other associated saved objects from Kibana. This command adjusts the downloaded saved objects according to package naming conventions (prefixes, unique IDs) and writes them locally into folders corresponding to saved object types (dashboard, visualization, map, etc.).`

func setupExportCommand() *cobraext.Command {
	exportDashboardCmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Export dashboards from Kibana",
		Long:  exportDashboardsLongDescription,
		RunE:  exportDashboardsCmd,
	}
	exportDashboardCmd.Flags().StringSliceP(cobraext.DashboardIDsFlagName, "d", nil, cobraext.DashboardIDsFlagDescription)
	exportDashboardCmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)
	exportDashboardCmd.Flags().Bool(cobraext.AllowSnapshotFlagName, false, cobraext.AllowSnapshotDescription)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export package assets",
		Long:  exportLongDescription,
	}
	cmd.AddCommand(exportDashboardCmd)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func exportDashboardsCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("Export Kibana dashboards")

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

	allowSnapshot, _ := cmd.Flags().GetBool(cobraext.AllowSnapshotFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.AllowSnapshotFlagName)
	}

	kibanaClient, err := kibana.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	kibanaVersion, err := kibanaClient.Version()
	if err != nil {
		return fmt.Errorf("can't get Kibana status information: %w", err)
	}

	if kibanaVersion.IsSnapshot() {
		message := fmt.Sprintf("exporting dashboards from a SNAPSHOT version of Kibana (%s) is discouraged. It could lead to invalid dashboards (for example if they use features that are reverted or modified before the final release)", kibanaVersion.Version())
		if !allowSnapshot {
			return fmt.Errorf("%s. --%s flag can be used to ignore this error", message, cobraext.AllowSnapshotFlagName)
		}
		fmt.Printf("Warning: %s\n", message)
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

	err = export.Dashboards(kibanaClient, dashboardIDs)
	if err != nil {
		return fmt.Errorf("dashboards export failed: %w", err)
	}

	cmd.Println("Done")
	return nil
}

func promptDashboardIDs(kibanaClient *kibana.Client) ([]string, error) {
	savedDashboards, err := kibanaClient.FindDashboards()
	if err != nil {
		return nil, fmt.Errorf("finding dashboards failed: %w", err)
	}

	if len(savedDashboards) == 0 {
		return []string{}, nil
	}

	dashboardsPrompt := &survey.MultiSelect{
		Message:  "Which dashboards would you like to export?",
		Options:  savedDashboards.Strings(),
		PageSize: 100,
	}

	var selectedOptions []string
	err = survey.AskOne(dashboardsPrompt, &selectedOptions, survey.WithValidator(survey.Required))
	if err != nil {
		return nil, err
	}

	var selected []string
	for _, option := range selectedOptions {
		for _, sd := range savedDashboards {
			if sd.String() == option {
				selected = append(selected, sd.ID)
			}
		}
	}
	return selected, nil
}
