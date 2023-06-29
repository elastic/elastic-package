// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/package-spec/v2/code/go/pkg/validator"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/docs"
	"github.com/elastic/elastic-package/internal/packages"
)

const lintLongDescription = `Use this command to validate the contents of a package using the package specification (see: https://github.com/elastic/package-spec).

The command ensures that the package is aligned with the package spec and the README file is up-to-date with its template (if present).`

func setupLintCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint the package",
		Long:  lintLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cobraext.ComposeCommandActions(cmd, args,
				lintCommandAction,
				validateSourceCommandAction,
			)
			if err != nil {
				return err
			}
			cmd.Println("Done")
			return nil
		},
	}

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func lintCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Lint the package")

	readmeFiles, err := docs.AreReadmesUpToDate()
	if err != nil {
		for _, f := range readmeFiles {
			if !f.UpToDate {
				cmd.Printf("%s is outdated. Rebuild the package with 'elastic-package build'\n%s", f.FileName, f.Diff)
			}
			if f.Error != nil {
				cmd.Printf("check if %s is up-to-date failed: %s\n", f.FileName, f.Error)
			}
		}
		return fmt.Errorf("checking readme files are up-to-date failed: %w", err)
	}
	return nil
}

func validateSourceCommandAction(cmd *cobra.Command, args []string) error {
	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return errors.New("package root not found")
	}
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}
	err = validator.ValidateFromPath(packageRootPath)
	if err != nil {
		return fmt.Errorf("linting package failed: %w", err)
	}

	return nil
}
