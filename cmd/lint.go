// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/docs"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/validation"
)

const lintLongDescription = `Use this command to validate the contents of a package using the package specification (see: https://github.com/elastic/package-spec).

The command ensures that the package is aligned with the package spec and the README file is up-to-date with its template (if present).`

func setupLintCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint the package",
		Long:  lintLongDescription,
		Args:  cobra.NoArgs,
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

	repositoryRoot, err := files.FindRepositoryRoot()
	if err != nil {
		return fmt.Errorf("locating repository root failed: %w", err)
	}
	defer repositoryRoot.Close()

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return fmt.Errorf("package root not found: %w", err)
	}

	readmeFiles, err := docs.AreReadmesUpToDate(repositoryRoot, packageRoot)
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
	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}
	errs, skipped := validation.ValidateAndFilterFromPath(packageRoot)
	if skipped != nil {
		logger.Infof("Skipped errors: %v", skipped)
	}
	if errs != nil {
		return fmt.Errorf("linting package failed: %w", errs)
	}
	return nil
}
