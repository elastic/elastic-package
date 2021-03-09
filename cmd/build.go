// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/docs"
	"github.com/elastic/elastic-package/internal/packages"
)

const buildLongDescription = `Use this command to build a package. Currently it supports only the "integration" package type.

Built packages are stored in the "build/" folder located at the root folder of the local Git repository checkout that contains your package folder. The command will also render the README file in your package folder if there is a corresponding template file present in "_dev/build/docs/README.md". All "_dev" directories under your package will be omitted.

Built packages are served up by the Elastic Package Registry running locally (see "elastic-package stack"). If you want a local package to be served up by the local Elastic Package Registry, make sure to build that package first using "elastic-package build".

Built packages can also be published to the global package registry service.

Context:
  package`

func setupBuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the package",
		Long:  buildLongDescription,
		RunE:  buildCommandAction,
	}
	return cmd
}

func buildCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Build the package")

	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return errors.New("package root not found")
	}
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}

	readmeFiles, err := ioutil.ReadDir(filepath.Join(packageRootPath, "_dev", "build", "docs"))
	if err != nil {
		return errors.Wrapf(err, "failed to return a list of directory entries from %s", packageRootPath)
	}

	for _, readme := range readmeFiles {
		fileName := readme.Name()
		target, err := docs.UpdateReadme(fileName)
		if err != nil {
			return errors.Wrapf(err, "updating %s file failed", fileName)
		}
		if target != "" {
			cmd.Printf("%s file rendered: %s\n", fileName, target)
		}

		target, err = builder.BuildPackage()
		if err != nil {
			return errors.Wrap(err, "building package failed")
		}
		cmd.Printf("Package built: %s\n", target)
	}

	cmd.Println("Done")
	return nil
}
