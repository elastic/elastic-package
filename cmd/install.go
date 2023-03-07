// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/package-spec/v2/code/go/pkg/validator"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"
)

const installLongDescription = `Use this command to install the package in Kibana.

The command uses Kibana API to install the package in Kibana. The package must be exposed via the Package Registry or built locally in zip format so they can be installed using --zip parameter. Zip packages can be installed directly in Kibana >= 8.7.0. More details in this [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/install_package.md).`

var semver8_7_0 = semver.MustParse("8.7.0")

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
	cmd.Flags().Bool(cobraext.BuildSkipValidationFlagName, false, cobraext.BuildSkipValidationFlagDescription)

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
	skipValidation, err := cmd.Flags().GetBool(cobraext.BuildSkipValidationFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.BuildSkipValidationFlagName)
	}

	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return errors.Wrap(err, "could not create kibana client")
	}
	kibanaVersion, err := kibanaClient.Version()
	if err != nil {
		return err
	}
	v, err := semver.NewVersion(kibanaVersion.Number)
	if err != nil {
		return fmt.Errorf("invalid Kibana version")
	}

	installer, err := newInstaller(zipPathFile, packageRootPath, v, skipValidation)
	if err != nil {
		return err
	}

	manifest, err := installer.manifest()
	if err != nil {
		return err
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

	return installer.install(cmd, manifest.Name, manifest.Version)
}

type packageInstaller interface {
	manifest() (*packages.PackageManifest, error)
	install(cmd *cobra.Command, name, version string) error
}

func newInstaller(zipPath, packageRootPath string, kibanaVersion *semver.Version, skipValidation bool) (packageInstaller, error) {
	if zipPath != "" {
		if kibanaVersion.LessThan(semver8_7_0) {
			return nil, fmt.Errorf("not supported uploading zip packages in Kibana %s", kibanaVersion)
		}
		if !skipValidation {
			logger.Debugf("Validating built .zip package (path: %s)", zipPath)
			err := validator.ValidateFromZip(zipPath)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid content found in built zip package")
			}
		}
		logger.Debug("Skip validation of the built .zip package")
		return zipPackage{zipPath: zipPath}, nil
	}
	if packageRootPath == "" {
		var found bool
		var err error
		packageRootPath, found, err = packages.FindPackageRoot()
		if !found {
			return nil, errors.New("package root not found")
		}
		if err != nil {
			return nil, errors.Wrap(err, "locating package root failed")
		}
	}

	if kibanaVersion.LessThan(semver8_7_0) {
		return localPackage{rootPath: packageRootPath}, nil
	}

	// build and install
	target, err := builder.BuildPackage(builder.BuildOptions{
		PackageRoot:    packageRootPath,
		CreateZip:      true,
		SignPackage:    false,
		SkipValidation: skipValidation,
	})
	if err != nil {
		return nil, errors.Wrap(err, "building package failed")
	}
	logger.Infof("Built package path: %s", target)

	return zipPackage{zipPath: target}, nil
}

type localPackage struct {
	rootPath string
}

func (l localPackage) manifest() (*packages.PackageManifest, error) {
	logger.Debugf("Reading package manifest from %s", l.rootPath)
	manifest, err := packages.ReadPackageManifestFromPackageRoot(l.rootPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading package manifest failed (path: %s)", l.rootPath)
	}
	return manifest, nil
}

func (l localPackage) install(cmd *cobra.Command, name, version string) error {
	aInstaller, err := installer.CreateForManifest(name, version)
	if err != nil {
		return errors.Wrap(err, "can't create the package installer")
	}

	cmd.Println("Install the package")
	return installPackage(cmd, aInstaller)
}

type zipPackage struct {
	zipPath string
}

func (z zipPackage) manifest() (*packages.PackageManifest, error) {
	logger.Debugf("Reading package manifest from %s", z.zipPath)
	manifest, err := packages.ReadPackageManifestFromZipPackage(z.zipPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading package manifest failed (path: %s)", z.zipPath)
	}
	return manifest, nil
}

func (z zipPackage) install(cmd *cobra.Command, name, version string) error {
	aInstaller, err := installer.CreateForZip(z.zipPath, name, version)
	if err != nil {
		return errors.Wrap(err, "can't create the package installer")
	}

	cmd.Printf("Install zip package: %s\n", z.zipPath)
	return installPackage(cmd, aInstaller)
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
