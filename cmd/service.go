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

const serviceLongDescription = `Use this command to boot up the service stack that can be observed with the package.

The command manages lifecycle of the service stack defined for the package ("_dev/deploy") for package development and testing purposes.`

func setupServiceCommand() *cobraext.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the stack",
		Args:  cobra.NoArgs,
		RunE:  upCommandAction,
	}
	upCommand.Flags().StringP(cobraext.DataStreamFlagName, "d", "", cobraext.DataStreamFlagDescription)
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

	var dataStreamPath string
	dataStreamFlag, _ := cmd.Flags().GetString(cobraext.DataStreamFlagName)
	if dataStreamFlag != "" {
		dataStreamPath = filepath.Join(packageRoot, "data_stream", dataStreamFlag)
	}

	variantFlag, _ := cmd.Flags().GetString(cobraext.VariantFlagName)

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

	_, serviceName := filepath.Split(packageRoot)
	err = service.BootUp(cmd.Context(), service.Options{
		Profile:            profile,
		ServiceName:        serviceName,
		PackageRootPath:    packageRoot,
		DevDeployDir:       system.DevDeployDir,
		DataStreamRootPath: dataStreamPath,
		Variant:            variantFlag,
		StackVersion:       stackVersion.Version(),
		AgentVersion:       stackVersion.Version(),
	})
	if err != nil {
		return fmt.Errorf("up command failed: %w", err)
	}
	return nil
}
