// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/service"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner/runners/system"
)

const serviceUpLongDescription = `Boot up the service stack defined for the package ("_dev/deploy") until it is interrupted.

By default, the service used is the one named after the package in the docker-compose file, with the name in the manifest of the package (not with the name of its folder), the same as in system tests. Use the --service flag to use a service with a different name.`

const serviceLongDescription = `Use this command to boot up the service stack that can be observed with the package.

The command manages lifecycle of the service stack defined for the package ("_dev/deploy") for package development and testing purposes.`

func setupServiceCommand() *cobraext.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the stack",
		Long:  serviceUpLongDescription,
		Args:  cobra.NoArgs,
		RunE:  upCommandAction,
	}
	upCommand.Flags().StringP(cobraext.DataStreamFlagName, "d", "", cobraext.DataStreamFlagDescription)
	upCommand.Flags().String(cobraext.ServiceFlagName, "", cobraext.ServiceFlagDescription)
	upCommand.Flags().String(cobraext.VariantFlagName, "", cobraext.VariantFlagDescription)

	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the service stack",
		Long:  serviceLongDescription,
	}
	cmd.AddCommand(upCommand)
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func upCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Boot up the service stack")

	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	var dataStreamRoot string
	dataStreamFlag, _ := cmd.Flags().GetString(cobraext.DataStreamFlagName)
	if dataStreamFlag != "" {
		dataStreamRoot = filepath.Join(packageRoot, "data_stream", dataStreamFlag)
	}

	variantFlag, _ := cmd.Flags().GetString(cobraext.VariantFlagName)
	serviceFlag, _ := cmd.Flags().GetString(cobraext.ServiceFlagName)

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	kibanaClient, err := stack.NewKibanaClientFromProfile(profile)
	if err != nil {
		return fmt.Errorf("cannot create Kibana client: %w", err)
	}
	stackVersion, err := kibanaClient.Version()
	if err != nil {
		return fmt.Errorf("cannot request Kibana version: %w", err)
	}

	serviceName, err := serviceNameToBootUp(serviceFlag, packageRoot)
	if err != nil {
		return err
	}

	err = service.BootUp(cmd.Context(), service.Options{
		Profile:        profile,
		ServiceName:    serviceName,
		PackageRoot:    packageRoot,
		DevDeployDir:   system.DevDeployDir,
		DataStreamRoot: dataStreamRoot,
		Variant:        variantFlag,
		StackVersion:   stackVersion.Version(),
	})
	if err != nil {
		return fmt.Errorf("up command failed: %w", err)
	}
	return nil
}

// serviceNameToBootUp returns the name of the service to boot up: the one set with the service flag, or
// by default the name of the package in its manifest.
func serviceNameToBootUp(serviceFlag, packageRoot string) (string, error) {
	if serviceFlag != "" {
		return serviceFlag, nil
	}

	packageName, err := packages.ReadPackageName(packageRoot)
	if err != nil {
		return "", fmt.Errorf("cannot determine the name of the service to boot up, use the --%s flag: %w", cobraext.ServiceFlagName, err)
	}
	return packageName, nil
}
