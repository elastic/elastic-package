// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/version"
)

const versionLongDescription = `Use this command to print the version of elastic-package that you have installed. This is especially useful when reporting bugs.`

func setupVersionCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Long:  versionLongDescription,
		Args:  cobra.NoArgs,
		RunE:  versionCommandAction,
	}

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func versionCommandAction(cmd *cobra.Command, args []string) error {
	fmt.Println(version.Version())
	return nil
}
