// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/kibana"
)

func setupExportCommand() *cobra.Command {
	exportDashboardCmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Export dashboards from Kibana",
		Long:  "Use dashboards subcommand to export dashboards with referenced objects from the Kibana instance.",
		RunE:  exportDashboardsCmd,
	}
	exportDashboardCmd.Flags().StringSliceP(cobraext.DashboardIDsFlagName, "d", nil, cobraext.DashboardIDsFlagDescriptions)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export package assets",
		Long:  "Use export command to export assets relevant for the package from the Elastic stack.",
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

	if len(dashboardIDs) == 0 {
		dashboardIDs, err = promptDashboardIDs()
		if err != nil {
			return errors.Wrap(err, "prompt for dashboard selection failed")
		}
	}

	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return errors.Wrap(err, "creating Kibana client failed")
	}

	err = export.Dashboards(kibanaClient, dashboardIDs)
	if err != nil {
		return errors.Wrap(err, "dashboards export failed")
	}

	cmd.Println("Done")
	return nil
}

func promptDashboardIDs() ([]string, error) {
	return nil, errors.New("not implemented yet")
}
