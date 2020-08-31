// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/logger"
)

// RootCmd creates and returns root cmd for elastic-package
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               "elastic-package",
		Short:             "elastic-package - Command line tool for developing Elastic Integrations",
		SilenceUsage:      true,
		PersistentPreRunE: processPersistentFlags,
	}
	rootCmd.PersistentFlags().BoolP(cobraext.VerboseFlagName, "v", false, cobraext.VerboseFlagDescription)

	rootCmd.AddCommand(
		setupBuildCommand(),
		setupCheckCommand(),
		setupStackCommand(),
		setupFormatCommand(),
		setupLintCommand(),
		setupPromoteCommand(),
		setupTestCommand(),
		setupVersionCommand())
	return rootCmd
}

func processPersistentFlags(cmd *cobra.Command, args []string) error {
	verbose, err := cmd.Flags().GetBool(cobraext.VerboseFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.VerboseFlagName)
	}

	if verbose {
		logger.EnableDebugMode()
	}
	return nil
}
