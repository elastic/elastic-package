// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/version"
)

var commands = []*cobraext.Command{
	setupBenchmarkCommand(),
	setupBuildCommand(),
	setupChangelogCommand(),
	setupCheckCommand(),
	setupCleanCommand(),
	setupCreateCommand(),
	setupDumpCommand(),
	setupEditCommand(),
	setupExportCommand(),
	setupFormatCommand(),
	setupInstallCommand(),
	setupLintCommand(),
	setupPromoteCommand(),
	setupProfilesCommand(),
	setupPublishCommand(),
	setupReportsCommand(),
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
	rootCmd.PersistentFlags().CountP(cobraext.VerboseFlagName, "v", cobraext.VerboseFlagDescription)
	rootCmd.PersistentFlags().String(cobraext.LogFormatFlagName, logger.DefaultFormatLabel, cobraext.LogFormatFlagDescription)

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
	verbosity, err := cmd.Flags().GetCount(cobraext.VerboseFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.VerboseFlagName)
	}

	LogFormat, err := cmd.Flags().GetString(cobraext.LogFormatFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.LogFormatFlagName)
	}

	opts := logger.LoggerOptions{
		Verbosity: verbosity,
		LogFormat: LogFormat,
	}

	err = logger.SetupLogger(opts)
	if err != nil {
		return fmt.Errorf("failed to setup logger: %w", err)
	}
	return nil
}

func checkVersionUpdate(cmd *cobra.Command, args []string) error {
	version.CheckUpdate(cmd.Context(), logger.Logger)
	return nil
}
