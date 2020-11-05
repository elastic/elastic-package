package cmd

import (
	"github.com/elastic/elastic-package/internal/kibana/dashboards"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/export"
)

func setupExportCommand() *cobra.Command {
	exportDashboardCmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Export dashboard from Kibana",
		Long:  "Use dashboard subcommand to export dashboard from the Kibana instance.",
		RunE:  exportDashboardCmd,
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

func exportDashboardCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("Export Kibana dashboard")

	dashboardIDs, err := cmd.Flags().GetStringSlice(cobraext.DashboardIDsFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.DashboardIDsFlagName)
	}

	kibanaDashboardsClient, err := dashboards.NewClient()
	if err != nil {
		return errors.Wrap(err, "creating Kibana Dashboards client failed")
	}

	err = export.Dashboards(kibanaDashboardsClient, dashboardIDs)
	if err != nil {
		return errors.Wrap(err, "dashboards export failed")
	}

	cmd.Println("Done")
	return nil
}
