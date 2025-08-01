// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/docs"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

const buildLongDescription = `Use this command to build a package.

Built packages are stored in the "build/" folder located at the root folder of the local Git repository checkout that contains your package folder. The command will also render the README file in your package folder if there is a corresponding template file present in "_dev/build/docs/README.md". All "_dev" directories under your package will be omitted. For details on how to generate and syntax of this README, see the [HOWTO guide](./docs/howto/add_package_readme.md).

Built packages are served up by the Elastic Package Registry running locally (see "elastic-package stack"). If you want a local package to be served up by the local Elastic Package Registry, make sure to build that package first using "elastic-package build".

Built packages can also be published to the global package registry service.

For details on how to enable dependency management, see the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/dependency_management.md).`

func setupBuildCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the package",
		Long:  buildLongDescription,
		Args:  cobra.NoArgs,
		RunE:  buildCommandAction,
	}
	cmd.Flags().Bool(cobraext.BuildZipFlagName, true, cobraext.BuildZipFlagDescription)
	cmd.Flags().Bool(cobraext.SignPackageFlagName, false, cobraext.SignPackageFlagDescription)
	cmd.Flags().Bool(cobraext.BuildSkipValidationFlagName, false, cobraext.BuildSkipValidationFlagDescription)
	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func buildCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Build the package")

	createZip, _ := cmd.Flags().GetBool(cobraext.BuildZipFlagName)
	signPackage, _ := cmd.Flags().GetBool(cobraext.SignPackageFlagName)
	skipValidation, _ := cmd.Flags().GetBool(cobraext.BuildSkipValidationFlagName)

	if signPackage && !createZip {
		return errors.New("can't sign the unzipped package, please use also the --zip switch")
	}

	if signPackage {
		err := files.VerifySignerConfiguration()
		if err != nil {
			return fmt.Errorf("can't verify signer configuration: %w", err)
		}
	}

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return fmt.Errorf("can't prepare build directory: %w", err)
	}
	logger.Debugf("Use build directory: %s", buildDir)

	targets, err := docs.UpdateReadmes(packageRoot, buildDir)
	if err != nil {
		return fmt.Errorf("updating files failed: %w", err)
	}

	for _, target := range targets {
		fileName := filepath.Base(target)
		cmd.Printf("%s file rendered: %s\n", fileName, target)
	}

	target, err := builder.BuildPackage(cmd.Context(), builder.BuildOptions{
		PackageRoot:    packageRoot,
		BuildDir:       buildDir,
		CreateZip:      createZip,
		SignPackage:    signPackage,
		SkipValidation: skipValidation,
	})
	if err != nil {
		return fmt.Errorf("building package failed: %w", err)
	}
	cmd.Printf("Package built: %s\n", target)

	cmd.Println("Done")
	return nil
}
