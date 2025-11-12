// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/packages"
)

const formatLongDescription = `Use this command to format the package files.

The formatter supports JSON and YAML format, and skips "ingest_pipeline" directories as it's hard to correctly format Handlebars template files. Formatted files are being overwritten.`

func setupFormatCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Format the package",
		Long:  formatLongDescription,
		Args:  cobra.NoArgs,
		RunE:  formatCommandAction,
	}
	cmd.Flags().BoolP(cobraext.FailFastFlagName, "f", false, cobraext.FailFastFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func formatCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Format the package")
	cwd, err := cobraext.Getwd(cmd)
	if err != nil {
		return err
	}

	packageRoot, err := packages.FindPackageRoot(cwd)
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	ff, err := cmd.Flags().GetBool(cobraext.FailFastFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FailFastFlagName)
	}

	err = formatter.Format(packageRoot, ff)
	if err != nil {
		return fmt.Errorf("formatting the integration failed (path: %s, failFast: %t): %w", packageRoot, ff, err)
	}

	cmd.Println("Done")
	return nil
}
