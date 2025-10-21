// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/tui"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	exportDashboardsLongDescription = `Use this command to export dashboards with referenced objects from the Kibana instance.

Use this command to download selected dashboards and other associated saved objects from Kibana. This command adjusts the downloaded saved objects according to package naming conventions (prefixes, unique IDs) and writes them locally into folders corresponding to saved object types (dashboard, visualization, map, etc.).`

	newDashboardOption = "Working on a new dashboard (show all available dashboards)"
)

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
		message := fmt.Sprintf("exporting dashboards from a SNAPSHOT version of Kibana (%s) is discouraged. It could lead to invalid dashboards (for example if they use features that are reverted or modified before the final release)", kibanaVersion.Version())
		if !allowSnapshot {
			return fmt.Errorf("%s. --%s flag can be used to ignore this error", message, cobraext.AllowSnapshotFlagName)
		}
		fmt.Printf("Warning: %s\n", message)
	}

	// Just query for dashboards if none were provided as flags
	if len(dashboardIDs) == 0 {
		packageRoot, err := packages.MustFindPackageRoot()
		if err != nil {
			return fmt.Errorf("locating package root failed: %w", err)
		}
		m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
		if err != nil {
			return fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
		}
		options := selectDashboardOptions{
			ctx:            cmd.Context(),
			kibanaClient:   kibanaClient,
			kibanaVersion:  kibanaVersion,
			defaultPackage: m.Name,
		}

		dashboardIDs, err = selectDashboardIDs(options)
		if err != nil {
			return fmt.Errorf("selecting dashboard IDs failed: %w", err)
		}

		if len(dashboardIDs) == 0 {
			fmt.Println("No dashboards were found in Kibana.")
			return nil
		}
	}

	err = export.Dashboards(cmd.Context(), kibanaClient, dashboardIDs)
	if err != nil {
		return fmt.Errorf("dashboards export failed: %w", err)
	}

	cmd.Println("Done")
	return nil
}

type selectDashboardOptions struct {
	ctx            context.Context
	kibanaClient   *kibana.Client
	kibanaVersion  kibana.VersionInfo
	defaultPackage string
}

// selectDashboardIDs prompts the user to select dashboards to export. It handles
// different flows depending on whether the Kibana instance is a Serverless environment or not.
// In non-Serverless environments, it prompts directly for dashboard selection.
// In Serverless environments, it first prompts to select an installed package or choose
// to export new dashboards, and then prompts for dashboard selection accordingly.
func selectDashboardIDs(options selectDashboardOptions) ([]string, error) {
	if options.kibanaVersion.BuildFlavor != kibana.ServerlessFlavor {
		// This method uses a deprecated API to search for saved objects.
		// And this API is not available in Serverless environments.
		dashboardIDs, err := promptDashboardIDsNonServerless(options.ctx, options.kibanaClient)
		if err != nil {
			return nil, fmt.Errorf("prompt for dashboard selection failed: %w", err)
		}
		return dashboardIDs, nil
	}

	installedPackage, err := promptPackagesInstalled(options.ctx, options.kibanaClient, options.defaultPackage)
	if err != nil {
		return nil, fmt.Errorf("prompt for package selection failed: %w", err)
	}

	if installedPackage == "" {
		fmt.Println("No installed packages were found in Kibana.")
		return nil, nil
	}

	if installedPackage == newDashboardOption {
		dashboardIDs, err := promptDashboardIDsServerless(options.ctx, options.kibanaClient)
		if err != nil {
			return nil, fmt.Errorf("prompt for dashboard selection failed: %w", err)
		}
		return dashboardIDs, nil
	}

	// As it can be installed just one version of a package in Elastic, we can split by '-' to get the name.
	// This package name will be used to get the data related to a package (kibana.GetPackage).
	installedPackageName, _, found := strings.Cut(installedPackage, "-")
	if !found {
		return nil, fmt.Errorf("invalid package name: %s", installedPackage)
	}

	dashboardIDs, err := promptPackageDashboardIDs(options.ctx, options.kibanaClient, installedPackageName)
	if err != nil {
		return nil, fmt.Errorf("prompt for package dashboard selection failed: %w", err)
	}

	return dashboardIDs, nil
}

func promptPackagesInstalled(ctx context.Context, kibanaClient *kibana.Client, defaultPackageName string) (string, error) {
	installedPackages, err := kibanaClient.FindInstalledPackages(ctx)
	if err != nil {
		return "", fmt.Errorf("finding installed packages failed: %w", err)
	}

	// First option is always to list all available dashboards even if they are not related
	// to any package. This is helpful in case the user is working on a new dashboard.
	options := []string{newDashboardOption}

	options = append(options, installedPackages.Strings()...)
	defaultOption := ""
	for _, ip := range installedPackages {
		if ip.Name == defaultPackageName {
			// set default package to the one matching the package in the current directory
			defaultOption = ip.String()
			break
		}
	}

	packagesPrompt := tui.NewSelect("Which packages would you like to export dashboards from?", options, defaultOption)

	var selectedOption string
	err = tui.AskOne(packagesPrompt, &selectedOption, tui.Required)
	if err != nil {
		return "", err
	}

	return selectedOption, nil
}

// promptPackageDashboardIDs prompts the user to select dashboards from the given package.
// It requires the package name to fetch the installed package information from Kibana.
func promptPackageDashboardIDs(ctx context.Context, kibanaClient *kibana.Client, packageName string) ([]string, error) {
	installedPackage, err := kibanaClient.GetPackage(ctx, packageName)
	if err != nil {
		return nil, fmt.Errorf("failed to get package status: %w", err)
	}
	if installedPackage.Status == "not_installed" {
		return nil, fmt.Errorf("package %s is not installed", packageName)
	}

	// get asset titles from IDs
	packageAssets := []packages.Asset{}
	for _, asset := range installedPackage.InstallationInfo.InstalledKibanaAssets {
		if asset.Type != "dashboard" {
			continue
		}

		packageAssets = append(packageAssets, packages.Asset{ID: asset.ID, Type: asset.Type})
	}

	assetsResponse, err := kibanaClient.GetDataFromPackageAssetIDs(ctx, packageAssets)
	if err != nil {
		return nil, fmt.Errorf("failed to get package assets: %w", err)
	}

	dashboardIDOptions := []string{}
	for _, asset := range assetsResponse {
		if asset.Type != "dashboard" {
			continue
		}
		dashboardIDOptions = append(dashboardIDOptions, asset.String())
	}

	if len(dashboardIDOptions) == 0 {
		return nil, fmt.Errorf("no dashboards found for package %s", packageName)
	}

	dashboardsPrompt := tui.NewMultiSelect("Which dashboards would you like to export?", dashboardIDOptions, []string{})
	dashboardsPrompt.SetPageSize(100)

	var selectedOptions []string
	err = tui.AskOne(dashboardsPrompt, &selectedOptions, tui.Required)
	if err != nil {
		return nil, err
	}

	var selectedIDs []string
	for _, option := range selectedOptions {
		for _, asset := range assetsResponse {
			if asset.String() == option {
				selectedIDs = append(selectedIDs, asset.ID)
			}
		}
	}

	return selectedIDs, nil
}

func promptDashboardIDsServerless(ctx context.Context, kibanaClient *kibana.Client) ([]string, error) {
	savedDashboards, err := kibanaClient.FindServerlessDashboards(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding dashboards failed: %w", err)
	}

	return promptDashboardIDs(savedDashboards)
}

func promptDashboardIDsNonServerless(ctx context.Context, kibanaClient *kibana.Client) ([]string, error) {
	savedDashboards, err := kibanaClient.FindDashboards(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding dashboards failed: %w", err)
	}

	return promptDashboardIDs(savedDashboards)
}

func promptDashboardIDs(savedDashboards kibana.DashboardSavedObjects) ([]string, error) {
	if len(savedDashboards) == 0 {
		return []string{}, nil
	}

	dashboardsPrompt := tui.NewMultiSelect("Which dashboards would you like to export?", savedDashboards.Strings(), []string{})
	dashboardsPrompt.SetPageSize(100)

	var selectedOptions []string
	err := tui.AskOne(dashboardsPrompt, &selectedOptions, tui.Required)
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
