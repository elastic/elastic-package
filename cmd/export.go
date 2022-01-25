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
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/kibana"
)

const exportLongDescription = `Use this command to export assets relevant for the package, e.g. Kibana dashboards.`

const exportDashboardsLongDescription = `Use this command to export dashboards with referenced objects from the Kibana instance.

Use this command to download selected dashboards and other associated saved objects from Kibana. This command adjusts the downloaded saved objects according to package naming conventions (prefixes, unique IDs) and writes them locally into folders corresponding to saved object types (dashboard, visualization, map, etc.).`

const exportInstalledObjectsLongDescription = `Use this command to export objects installed by Fleet as part of a package.

Use this command as a exploratory tool to export objects as they are installed by Fleet when installing a package. Exported objects are stored in files as they are in Elasticsearch or Kibana, without any processing.`

func setupExportCommand() *cobraext.Command {
	exportDashboardCmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Export dashboards from Kibana",
		Long:  exportDashboardsLongDescription,
		RunE:  exportDashboardsCmd,
	}
	exportDashboardCmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)
	exportDashboardCmd.Flags().StringSliceP(cobraext.DashboardIDsFlagName, "d", nil, cobraext.DashboardIDsFlagDescription)

	exportInstalledObjectsCmd := &cobra.Command{
		Use:   "installed-objects",
		Short: "Export installed Elasticsearch objects",
		Long:  exportInstalledObjectsLongDescription,
		RunE:  exportInstalledObjectsCmd,
	}
	exportInstalledObjectsCmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)
	exportInstalledObjectsCmd.Flags().StringP(cobraext.ExportPackageFlagName, "p", "", cobraext.ExportPackageFlagDescription)
	exportInstalledObjectsCmd.Flags().StringP(cobraext.ExportOutputFlagName, "o", "", cobraext.ExportOutputFlagDescription)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export package objects",
		Long:  exportLongDescription,
	}

	cmd.AddCommand(exportDashboardCmd)
	cmd.AddCommand(exportInstalledObjectsCmd)

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

	kibanaClient, err := kibana.NewClient(opts...)
	if err != nil {
		return errors.Wrap(err, "can't create Kibana client")
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

func exportInstalledObjectsCmd(cmd *cobra.Command, args []string) error {
	cmd.Println("Export Installed objects")

	packageName, err := cmd.Flags().GetString(cobraext.ExportPackageFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ExportPackageFlagName)
	}

	outputPath, err := cmd.Flags().GetString(cobraext.ExportOutputFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ExportOutputFlagName)
	}

	client, err := elasticsearch.Client()
	if err != nil {
		return errors.Wrap(err, "failed to initialize Elasticsearch client")
	}

	dataStreams, err := export.DataStreamsForPackage(cmd.Context(), client.API, packageName)
	if err != nil {
		return errors.Wrapf(err, "failed to obtain data streams for package %s", packageName)
	}
	if len(dataStreams) == 0 {
		cmd.Printf("No data streams found for package %s, is it installed?\n", packageName)
		return nil
	}

	cmd.Printf("export dir: %s\n", outputPath)
	cmd.Printf("%d data streams to export\n", len(dataStreams))

	// ILM Policies
	// Composable Templates
	// Index Templates
	// Ingest Pipelines
	// Transforms
	cmd.Println("Done")
	return nil
}
