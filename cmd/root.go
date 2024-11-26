// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/telemetry"
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

var (
	tracer       trace.Tracer
	otelShutdown func(context.Context) error
	cmdSpan      trace.Span
)

// RootCmd creates and returns root cmd for elastic-package
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "elastic-package",
		Short:        "elastic-package - Command line tool for developing Elastic Integrations",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			versionInfo := version.CommitHash
			if version.Tag != "" {
				versionInfo = version.Tag
			}
			shutdown, err := telemetry.SetupOTelSDK(cmd.Context(), versionInfo)
			if err != nil {
				return fmt.Errorf("failed to set up OpenTelemetry: %w", err)
			}

			tracer = otel.Tracer("elastic.co./elastic/elastic-package")

			otelShutdown = shutdown

			// wrap the whole command in a Span
			_, span := telemetry.StartSpanForCommand(tracer, cmd)
			cmdSpan = span

			return cobraext.ComposeCommandActions(cmd, args,
				processPersistentFlags,
				checkVersionUpdate,
			)
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			cmdSpan.End()

			if otelShutdown != nil {
				err := otelShutdown(cmd.Context())
				if err != nil {
					// we don't want to fail the process if telemetry can't be sent
					logger.Errorf("Failed to shut down OpenTelemetry: %s", err)
				}
			}

			return nil
		},
	}
	rootCmd.PersistentFlags().BoolP(cobraext.VerboseFlagName, cobraext.VerboseFlagShorthand, false, cobraext.VerboseFlagDescription)
	rootCmd.PersistentFlags().StringP(cobraext.ChangeDirectoryFlagName, cobraext.ChangeDirectoryFlagShorthand, "", cobraext.ChangeDirectoryFlagDescription)

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

	changeDirectory, err := cmd.Flags().GetString(cobraext.ChangeDirectoryFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ChangeDirectoryFlagName)
	}
	if changeDirectory != "" {
		err := os.Chdir(changeDirectory)
		if err != nil {
			return fmt.Errorf("failed to change directory: %w", err)
		}
		logger.Debugf("Running command in directory \"%s\"", changeDirectory)
	}

	return nil
}

func checkVersionUpdate(cmd *cobra.Command, args []string) error {
	version.CheckUpdate(cmd.Context())
	return nil
}
