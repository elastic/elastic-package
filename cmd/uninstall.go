// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/pkg/packages"
	"github.com/elastic/elastic-package/pkg/packages/installer"
)

const uninstallLongDescription = `Use this command to uninstall the package in Kibana.

The command uses Kibana API to uninstall the package in Kibana. The package must be exposed via the Package Registry.`

func setupUninstallCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the package",
		Long:  uninstallLongDescription,
		Args:  cobra.NoArgs,
		RunE:  uninstallCommandAction,
	}
	cmd.Flags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func uninstallCommandAction(cmd *cobra.Command, args []string) error {
	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return errors.New("package root not found")
	}
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	kibanaClient, err := stack.NewKibanaClientFromProfile(profile)
	if err != nil {
		return fmt.Errorf("could not create kibana client: %w", err)
	}
	packageInstaller, err := installer.CreateForManifest(kibanaClient, packageRootPath)
	if err != nil {
		return fmt.Errorf("can't create the package installer: %w", err)
	}

	// Uninstall the package
	cmd.Println("Uninstall the package")
	err = packageInstaller.Uninstall()
	if err != nil {
		return fmt.Errorf("can't uninstall the package: %w", err)
	}
	cmd.Println("Done")
	return nil
}
