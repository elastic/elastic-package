// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/docs"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

const buildLongDescription = `Use this command to build a package. Currently it supports only the "integration" package type.

Built packages are stored in the "build/" folder located at the root folder of the local Git repository checkout that contains your package folder. The command will also render the README file in your package folder if there is a corresponding template file present in "_dev/build/docs/README.md". All "_dev" directories under your package will be omitted.

Built packages are served up by the Elastic Package Registry running locally (see "elastic-package stack"). If you want a local package to be served up by the local Elastic Package Registry, make sure to build that package first using "elastic-package build".

Built packages can also be published to the global package registry service.

For details on how to enable dependency management, see the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/dependency_management.md).`

func setupBuildCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the package",
		Long:  buildLongDescription,
		RunE:  buildCommandAction,
	}
	cmd.Flags().Bool(cobraext.BuildZipFlagName, false, cobraext.BuildZipFlagDescription)
	cmd.Flags().Bool(cobraext.SignPackageFlagName, false, cobraext.SignPackageFlagDescription)
	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func buildCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Build the package")

	createZip, _ := cmd.Flags().GetBool(cobraext.BuildZipFlagName)
	signPackage, _ := cmd.Flags().GetBool(cobraext.SignPackageFlagName)

	if signPackage && !createZip {
		return errors.New("can't sign the unzipped package, please use also the --zip switch")
	}

	if signPackage {
		err := files.VerifySignerConfiguration()
		if err != nil {
			return errors.Wrap(err, "can't verify signer configuration")
		}
	}

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}

	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return errors.Wrap(err, "can't prepare build directory")
	}
	logger.Debugf("Use build directory: %s", buildDir)

	targets, err := docs.UpdateReadmes(packageRoot)
	if err != nil {
		return errors.Wrap(err, "updating files failed")
	}

	for _, target := range targets {
		splitTarget := strings.Split(target, "/")
		cmd.Printf("%s file rendered: %s\n", splitTarget[len(splitTarget)-1], target)
	}

	target, err := builder.BuildPackage(builder.BuildOptions{
		PackageRoot: packageRoot,
		CreateZip:   createZip,
		SignPackage: signPackage,
	})
	if err != nil {
		return errors.Wrap(err, "building package failed")
	}
	cmd.Printf("Package built: %s\n", target)

	cmd.Println("Done")
	return nil
}
