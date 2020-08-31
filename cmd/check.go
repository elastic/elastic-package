// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
)

func setupCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check the package",
		Long:  "Use check command to verify if the package is correct in terms of formatting, validation and building.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cobraext.ComposeCommandActions(cmd, args,
				formatCommandAction,
				lintCommandAction,
				buildCommandAction,
			)
			if err != nil {
				return errors.Wrap(err, "checking package failed")
			}
			return nil
		},
	}
	cmd.PersistentFlags().BoolP(cobraext.FailFastFlagName, "f", true, cobraext.FailFastFlagDescription)
	return cmd
}
