// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/builder"
)

func setupBuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the package",
		Long:  "Use build command to build the package.",
		RunE:  buildCommandAction,
	}
	return cmd
}

func buildCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Build the package")

	err := builder.BuildPackage()
	if err != nil {
		return errors.Wrap(err, "building package failed")
	}

	cmd.Println("Done")
	return nil
}
