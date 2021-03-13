// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/elastic/package-spec/code/go/pkg/validator"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/docs"
	"github.com/elastic/elastic-package/internal/packages"
)

const lintLongDescription = `Use this command to validate the contents of a package using the package specification (see: https://github.com/elastic/package-spec).

The command ensures that the package is aligned with the package spec and the README file is up-to-date with its template (if present).

Context:
  package`

func setupLintCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint the package",
		Long:  lintLongDescription,
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

	errNames, notOKNames, err := docs.AreReadmesUpToDate()
	if err != nil {
		return errors.Wrap(err, "can't check if readme files are up-to-date")
	}

	if errNames != "" {
		return errors.Wrapf(err, "can't check if %sare up-to-date", errNames)
	}

	if notOKNames != "" {
		return fmt.Errorf("%sare outdated. Rebuild the package with 'elastic-package build'", notOKNames)
	}

	err = validator.ValidateFromPath(packageRootPath)
	if err != nil {
		return errors.Wrap(err, "linting package failed")
	}

	cmd.Println("Done")
	return nil
}
