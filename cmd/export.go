// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/install"
)

const exportLongDescription = `Use this command to export assets relevant for the package, e.g. Kibana dashboards.`

func setupExportCommand() *cobraext.Command {
	exportDashboardCmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Export dashboards from Kibana",
		Long:  exportDashboardsLongDescription,
		Args:  cobra.NoArgs,
		RunE:  exportDashboardsCmd,
	}
	exportDashboardCmd.Flags().StringSliceP(cobraext.DashboardIDsFlagName, "d", nil, cobraext.DashboardIDsFlagDescription)
	exportDashboardCmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)
	exportDashboardCmd.Flags().Bool(cobraext.AllowSnapshotFlagName, false, cobraext.AllowSnapshotDescription)

	exportIngestPipelinesCmd := &cobra.Command{
		Use:   "ingest-pipelines",
		Short: "Export ingest pipelines from Elasticsearch",
		Long:  exportIngestPipelinesLongDescription,
		Args:  cobra.NoArgs,
		RunE:  exportIngestPipelinesCmd,
	}

	exportIngestPipelinesCmd.Flags().StringSliceP(cobraext.IngestPipelineIDsFlagName, "d", nil, cobraext.IngestPipelineIDsFlagDescription)
	exportIngestPipelinesCmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)
	exportIngestPipelinesCmd.Flags().Bool(cobraext.AllowSnapshotFlagName, false, cobraext.AllowSnapshotDescription)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export package assets",
		Long:  exportLongDescription,
	}
	cmd.AddCommand(exportDashboardCmd)
	cmd.AddCommand(exportIngestPipelinesCmd)
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}
