// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
)

const serviceLongDescription = `Use this command to boot up the service stack that can be observed with the package.

The command manages lifecycle of the service stack defined for the package ("_dev/deploy") for package development and testing purposes.`

func setupServiceCommand() *cobraext.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the stack",
		RunE:  upCommandAction,
	}
	upCommand.Flags().BoolP(cobraext.DaemonModeFlagName, "d", false, cobraext.DaemonModeFlagDescription)

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
	return nil
}
