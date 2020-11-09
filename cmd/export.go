package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/kibana/dashboards"
)

func setupExportCommand() *cobra.Command {
	exportDashboardCmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Export dashboards from Kibana",
		Long:  "Use dashboards subcommand to export dashboards with referenced objects from the Kibana instance.",
		RunE:  exportDashboardsCmd,
	}
	exportDashboardCmd.Flags().StringSliceP(cobraext.DashboardIDsFlagName, "d", nil, cobraext.DashboardIDsFlagDescriptions)
	exportDashboardCmd.MarkFlagRequired(cobraext.DashboardIDsFlagName)

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
