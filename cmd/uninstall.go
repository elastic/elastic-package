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

const uninstallLongDescription = `Use this command to uninstall the package in Kibana.

The command uses Kibana API to uninstall the package in Kibana. The package must be exposed via the Package Registry.`

func setupUninstallCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the package",
		Long:  uninstallLongDescription,
		RunE:  uninstallCommandAction,
	}

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func uninstallCommandAction(cmd *cobra.Command, args []string) error {
	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return fmt.Errorf("package root not found")
	}
	if err != nil {
		return fmt.Errorf("locating package root failed: %s", err)
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %s", packageRootPath, err)
	}

	packageInstaller, err := installer.CreateForManifest(*m)
	if err != nil {
		return fmt.Errorf("can't create the package installer: %s", err)
	}

	// Uninstall the package
	cmd.Println("Uninstall the package")
	err = packageInstaller.Uninstall()
	if err != nil {
		return fmt.Errorf("can't uninstall the package: %s", err)
	}
	cmd.Println("Done")
	return nil
}
