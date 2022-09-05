// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
)

const checkLongDescription = `Use this command to verify if the package is correct in terms of formatting, validation and building.

It will execute the format, lint, and build commands all at once, in that order.`

func setupCheckCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check the package",
		Long:  checkLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cobraext.ComposeCommands(args,
				setupFormatCommand(),
				setupLintCommand(),
				setupBuildCommand(),
			)
			if err != nil {
				return errors.Wrap(err, "checking package failed")
			}
			return nil
		},
	}
	cmd.PersistentFlags().BoolP(cobraext.FailFastFlagName, "f", true, cobraext.FailFastFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}
