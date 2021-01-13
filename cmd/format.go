// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/packages"
)

const formatLongDescription = `Use this command to format the package files.

The formatter supports JSON and YAML format, and skips "ingest_pipeline" directories as it's hard to correctly format Handlebars template files. Formatted files are being overwritten.

Context:
  package`

func setupFormatCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Format the package",
		Long:  formatLongDescription,
		RunE:  formatCommandAction,
	}
	cmd.Flags().BoolP(cobraext.FailFastFlagName, "f", false, cobraext.FailFastFlagDescription)
	return cmd
}

func formatCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Format the package")

	packageRoot, found, err := packages.FindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}
	if !found {
		return errors.New("package root not found")
	}

	ff, err := cmd.Flags().GetBool(cobraext.FailFastFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FailFastFlagName)
	}

	err = formatter.Format(packageRoot, ff)
	if err != nil {
		return errors.Wrapf(err, "formatting the integration failed (path: %s, failFast: %t)", packageRoot, ff)
	}

	cmd.Println("Done")
	return nil
}
