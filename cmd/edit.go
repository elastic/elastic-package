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

const editLongDescription = `Use this command to edit assets relevant for the package, e.g. Kibana dashboards.`

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
