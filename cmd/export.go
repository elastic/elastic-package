package cmd

import "github.com/spf13/cobra"

func setupExportCommand() *cobra.Command {
	exportDashboardCmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Export dashboard from Kibana",
		Long:  "Use dashboard subcommand to export dashboard from the Kibana instance.",
		RunE:  exportDashboardCmd,
	}

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
	cmd.Println("Done")
	return nil
}
