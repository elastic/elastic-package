// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"
)

const installLongDescription = `Use this command to install the package in Kibana.

The command uses Kibana API to install the package in Kibana. The package must be exposed via the Package Registry or built locally in zip format so they can be installed using --zip parameter. Zip packages can be installed directly in Kibana >= 8.7.0.`

func setupInstallCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the package",
		Long:  installLongDescription,
		RunE:  installCommandAction,
	}
	cmd.Flags().StringSliceP(cobraext.CheckConditionFlagName, "c", nil, cobraext.CheckConditionFlagDescription)
	cmd.Flags().StringP(cobraext.PackageRootFlagName, cobraext.PackageRootFlagShorthand, "", cobraext.PackageRootFlagDescription)
	cmd.Flags().StringP(cobraext.ZipPackageFilePathFlagName, cobraext.ZipPackageFilePathFlagShorthand, "", cobraext.ZipPackageFilePathFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func installCommandAction(cmd *cobra.Command, _ []string) error {
	zipPathFile, err := cmd.Flags().GetString(cobraext.ZipPackageFilePathFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ZipPackageFilePathFlagName)
	}
	packageRootPath, err := cmd.Flags().GetString(cobraext.PackageRootFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageRootFlagName)
	}
	if zipPathFile == "" && packageRootPath == "" {
		var found bool
		packageRootPath, found, err = packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return errors.Wrap(err, "locating package root failed")
		}
	}
	var manifest *packages.PackageManifest
	if zipPathFile != "" {
		logger.Debugf("Reading package manifest from %s", zipPathFile)
		manifest, err = packages.ReadPackageManifestFromZipPackage(zipPathFile)
		if err != nil {
			return errors.Wrapf(err, "reading package manifest failed (path: %s)", zipPathFile)
		}
	} else {
		logger.Debugf("Reading package manifest from %s", packageRootPath)
		manifest, err = packages.ReadPackageManifestFromPackageRoot(packageRootPath)
		if err != nil {
			return errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRootPath)
		}
	}

	// Check conditions
	keyValuePairs, err := cmd.Flags().GetStringSlice(cobraext.CheckConditionFlagName)
	if err != nil {
		return errors.Wrap(err, "can't process check-condition flag")
	}
	if len(keyValuePairs) > 0 {
		cmd.Println("Check conditions for package")
		err = packages.CheckConditions(*manifest, keyValuePairs)
		if err != nil {
			return errors.Wrap(err, "checking conditions failed")
		}
		cmd.Println("Requirements satisfied - the package can be installed.")
		cmd.Println("Done")
		return nil
	}

	if zipPathFile != "" {
		return installZipPackage(cmd, zipPathFile, manifest)
	}

	return installLocalPackage(cmd, manifest)
}

func installLocalPackage(cmd *cobra.Command, m *packages.PackageManifest) error {
	packageInstaller, err := installer.CreateForManifest(*m)
	if err != nil {
		return errors.Wrap(err, "can't create the package installer")
	}

	cmd.Println("Install the package")
	return installPackage(cmd, packageInstaller)
}

func installZipPackage(cmd *cobra.Command, zipPath string, m *packages.PackageManifest) error {

	packageInstaller, err := installer.CreateForZip(zipPath, *m)
	if err != nil {
		return errors.Wrap(err, "can't create the package installer")
	}

	cmd.Printf("Install zip package: %s\n", zipPath)
	return installPackage(cmd, packageInstaller)
}

func installPackage(cmd *cobra.Command, packageInstaller installer.Installer) error {
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
