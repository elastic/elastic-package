// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/installer"
)

const installLongDescription = `Use this command to install the package in Kibana.

The command uses Kibana API to install the package in Kibana. The package must be exposed via the Package Registry.

Context:
  package`

func setupInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the package",
		Long:  installLongDescription,
		RunE:  installCommandAction,
	}
	return cmd
}

func installCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Install the package")

	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return errors.New("package root not found")
	}
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}

	_, err = installer.Install(packageRootPath)
	if err != nil {
		return errors.Wrap(err, "can't install the package")
	}
	cmd.Println("Done")
	return nil
}
