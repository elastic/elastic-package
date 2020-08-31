// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/elastic/package-spec/code/go/pkg/validator"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
)

func setupLintCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint the package",
		Long:  "Use lint command to lint the package files.",
		RunE:  lintCommandAction,
	}
	return cmd
}

func lintCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Lint the package")

	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return errors.New("package root not found")
	}
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}

	err = validator.ValidateFromPath(packageRootPath)
	if err != nil {
		return errors.Wrap(err, "linting package failed")
	}

	cmd.Println("Done")
	return nil
}
