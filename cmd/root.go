// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/version"
)

var commands = []*cobraext.Command{
	setupBuildCommand(),
	setupCheckCommand(),
	setupCleanCommand(),
	setupCreateCommand(),
	setupDumpCommand(),
	setupExportCommand(),
	setupFormatCommand(),
	setupInstallCommand(),
	setupLintCommand(),
	setupProfilesCommand(),
	setupPromoteCommand(),
	setupPublishCommand(),
	setupServiceCommand(),
	setupStackCommand(),
	setupStatusCommand(),
	setupTestCommand(),
	setupUninstallCommand(),
	setupVersionCommand(),
}

// RootCmd creates and returns root cmd for elastic-package
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "elastic-package",
		Short:        "elastic-package - Command line tool for developing Elastic Integrations",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cobraext.ComposeCommandActions(cmd, args,
				processPersistentFlags,
				checkVersionUpdate,
			)
		},
	}
	rootCmd.PersistentFlags().BoolP(cobraext.VerboseFlagName, "v", false, cobraext.VerboseFlagDescription)

	for _, cmd := range commands {
		rootCmd.AddCommand(cmd.Command)
	}
	return rootCmd
}

// Commands returns the list of commands that have been setup for elastic-package.
func Commands() []*cobraext.Command {
	sort.SliceStable(commands, func(i, j int) bool {
		return commands[i].Name() < commands[j].Name()
	})

	return commands
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

func checkVersionUpdate(cmd *cobra.Command, args []string) error {
	version.CheckUpdate()
	return nil
}
