// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"
)

const installLongDescription = `Use this command to install the package in Kibana.

The command uses Kibana API to install the package in Kibana. The package must be exposed via the Package Registry.`

func setupInstallCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the package",
		Long:  installLongDescription,
		RunE:  installCommandAction,
	}
	cmd.Flags().StringSliceP(cobraext.CheckConditionFlagName, "c", nil, cobraext.CheckConditionFlagDescription)
	cmd.Flags().StringP(cobraext.PackageRootFlagName, cobraext.PackageRootFlagShorthand, "", cobraext.PackageRootFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func installCommandAction(cmd *cobra.Command, _ []string) error {
	packageRootPath, err := cmd.Flags().GetString(cobraext.PackageRootFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageRootFlagName)
	}
	if packageRootPath == "" {
		var found bool
		packageRootPath, found, err = packages.FindPackageRoot()
		if !found {
			return fmt.Errorf("package root not found")
		}
		if err != nil {
			return fmt.Errorf("locating package root failed: %s", err)
		}
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %s", packageRootPath, err)
	}

	// Check conditions
	keyValuePairs, err := cmd.Flags().GetStringSlice(cobraext.CheckConditionFlagName)
	if err != nil {
		return fmt.Errorf("can't process check-condition flag: %s", err)
	}
	if len(keyValuePairs) > 0 {
		cmd.Println("Check conditions for package")
		err = packages.CheckConditions(*m, keyValuePairs)
		if err != nil {
			return fmt.Errorf("checking conditions failed: %s", err)
		}
		cmd.Println("Requirements satisfied - the package can be installed.")
		cmd.Println("Done")
		return nil
	}

	packageInstaller, err := installer.CreateForManifest(*m)
	if err != nil {
		return fmt.Errorf("can't create the package installer: %s", err)
	}

	// Install the package
	cmd.Println("Install the package")
	installedPackage, err := packageInstaller.Install()
	if err != nil {
		return fmt.Errorf("can't install the package: %s", err)
	}

	cmd.Println("Installed assets:")
	for _, asset := range installedPackage.Assets {
		cmd.Printf("- %s (type: %s)\n", asset.ID, asset.Type)
	}
	cmd.Println("Done")
	return nil
}
