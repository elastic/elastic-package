// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/kibana"
)

const exportLongDescription = `Use this command to export assets relevant for the package, e.g. Kibana dashboards.

Context:
  package`

const exportDashboardsLongDescription = `Use this command to export dashboards with referenced objects from the Kibana instance.

Use this command to download selected dashboards and other associated saved objects from Kibana. This command adjusts the downloaded saved objects according to package naming conventions (prefixes, unique IDs) and writes them locally into folders corresponding to saved object types (dashboard, visualization, map, etc.).

Context:
  package`

func setupExportCommand() *cobra.Command {
	exportDashboardCmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Export dashboards from Kibana",
		Long:  exportDashboardsLongDescription,
		RunE:  exportDashboardsCmd,
	}
	exportDashboardCmd.Flags().StringSliceP(cobraext.DashboardIDsFlagName, "d", nil, cobraext.DashboardIDsFlagDescriptions)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export package assets",
		Long:  exportLongDescription,
	}
	cmd.AddCommand(exportDashboardCmd)
	return cmd
}

func exportDashboardsCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("Export Kibana dashboards")

	dashboardIDs, err := cmd.Flags().GetStringSlice(cobraext.DashboardIDsFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.DashboardIDsFlagName)
	}

	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return errors.Wrap(err, "creating Kibana client failed")
	}

	if len(dashboardIDs) == 0 {
		dashboardIDs, err = promptDashboardIDs(kibanaClient)
		if err != nil {
			return errors.Wrap(err, "prompt for dashboard selection failed")
		}

		if len(dashboardIDs) == 0 {
			fmt.Println("No dashboards were found in Kibana.")
			return nil
		}
	}

	err = export.Dashboards(kibanaClient, dashboardIDs)
	if err != nil {
		return errors.Wrap(err, "dashboards export failed")
	}

	cmd.Println("Done")
	return nil
}

func promptDashboardIDs(kibanaClient *kibana.Client) ([]string, error) {
	savedDashboards, err := kibanaClient.FindDashboards()
	if err != nil {
		return nil, errors.Wrap(err, "finding dashboards failed")
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
