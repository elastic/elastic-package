// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
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
	cmd.Flags().StringP(cobraext.InstallZipPackageFlagName, "", "", cobraext.InstallZipPackageFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func installCommandAction(cmd *cobra.Command, _ []string) error {
	zipPathFile, err := cmd.Flags().GetString(cobraext.InstallZipPackageFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.InstallZipPackageFlagName)
	}
	if zipPathFile != "" {
		return installZipPackage(cmd, zipPathFile)
	}

	packageRootPath, err := cmd.Flags().GetString(cobraext.PackageRootFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageRootFlagName)
	}
	if packageRootPath == "" {
		var found bool
		packageRootPath, found, err = packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return errors.Wrap(err, "locating package root failed")
		}
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRootPath)
	}

	// Check conditions
	keyValuePairs, err := cmd.Flags().GetStringSlice(cobraext.CheckConditionFlagName)
	if err != nil {
		return errors.Wrap(err, "can't process check-condition flag")
	}
	if len(keyValuePairs) > 0 {
		cmd.Println("Check conditions for package")
		err = packages.CheckConditions(*m, keyValuePairs)
		if err != nil {
			return errors.Wrap(err, "checking conditions failed")
		}
		cmd.Println("Requirements satisfied - the package can be installed.")
		cmd.Println("Done")
		return nil
	}

	return installLocalPackage(cmd, m)
}

func installLocalPackage(cmd *cobra.Command, m *packages.PackageManifest) error {
	packageInstaller, err := installer.CreateForManifest(*m)
	if err != nil {
		return errors.Wrap(err, "can't create the package installer")
	}

	return installPackage(cmd, packageInstaller)
}

func installZipPackage(cmd *cobra.Command, zipPath string) error {
	cmd.Printf("Install zip package: %s\n", zipPath)

	packageInstaller, err := installer.CreateForZip(zipPath)
	if err != nil {
		return errors.Wrap(err, "can't create the package installer")
	}

	return installPackage(cmd, packageInstaller)
}

func installPackage(cmd *cobra.Command, packageInstaller installer.Installer) error {
	cmd.Println("Install the package")
	installedPackage, err := packageInstaller.Install()
	if err != nil {
		return errors.Wrap(err, "can't install the package")
	}

	cmd.Println("Installed assets:")
	for _, asset := range installedPackage.Assets {
		cmd.Printf("- %s (type: %s)\n", asset.ID, asset.Type)
	}
	cmd.Println("Done")
	return nil
}
