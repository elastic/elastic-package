// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/service"
)

const serviceLongDescription = `Use this command to boot up the service stack that can be observed with the package.

The command manages lifecycle of the service stack defined for the package ("_dev/deploy") for package development and testing purposes.`

func setupServiceCommand() *cobraext.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the stack",
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

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func upCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Boot up the service stack")

	packageRoot, found, err := packages.FindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}
	if !found {
		return errors.New("package root not found")
	}

	var dataStreamPath string
	dataStreamFlag, _ := cmd.Flags().GetString(cobraext.DataStreamFlagName)
	if dataStreamFlag != "" {
		dataStreamPath = filepath.Join(packageRoot, "data_stream", dataStreamFlag)
	}

	variantFlag, _ := cmd.Flags().GetString(cobraext.VariantFlagName)

	_, serviceName := filepath.Split(packageRoot)
	err = service.BootUp(service.Options{
		ServiceName:        serviceName,
		PackageRootPath:    packageRoot,
		DataStreamRootPath: dataStreamPath,
		Variant:            variantFlag,
	})
	if err != nil {
		return errors.Wrap(err, "up command failed")
	}
	return nil
}
