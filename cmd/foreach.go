// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/filter"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
)

const foreachLongDescription = `[Technical Preview]
Execute a command for each package matching the given filter criteria.

This command combines filtering capabilities with command execution, allowing you to run any elastic-package subcommand across multiple packages in a single operation.

The command uses the same filter flags as the 'filter' command to select packages, then executes the specified subcommand for each matched package.`

// getAllowedSubCommands returns the list of allowed subcommands for the foreach command.
func getAllowedSubCommands() []string {
	return []string{
		"build",
		"check",
		"changelog",
		"clean",
		"format",
		"install",
		"lint",
		"test",
		"uninstall",
	}
}

func setupForeachCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "foreach [flags] -- <SUBCOMMAND>",
		Short: "Execute a command for filtered packages [Technical Preview]",
		Long:  fmt.Sprintf(foreachLongDescription+"\n\nAllowed subcommands:\n%s", strings.Join(getAllowedSubCommands(), ", ")),
		Example: `  # Run system tests for packages with specific inputs
  elastic-package foreach --input tcp,udp -- test system -g`,
		RunE: foreachCommandAction,
		Args: cobra.MinimumNArgs(1),
	}

	// Add filter flags
	filter.SetFilterFlags(cmd)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func foreachCommandAction(cmd *cobra.Command, args []string) error {
	if err := validateSubCommand(args[0]); err != nil {
		return fmt.Errorf("validating sub command failed: %w", err)
	}

	// reuse filterPackage from cmd/filter.go
	filtered, err := filterPackage(cmd)
	if err != nil {
		return fmt.Errorf("filtering packages failed: %w", err)
	}

	errors := multierror.Error{}

	for _, pkg := range filtered {
		rootCmd := cmd.Root()
		rootCmd.SetArgs(append(args, "--change-directory", pkg.Path))
		if err := rootCmd.Execute(); err != nil {
			errors = append(errors, err)
		}
	}

	logger.Infof("Successfully executed command for %d packages", len(filtered)-len(errors))

	if errors.Error() != "" {
		logger.Errorf("Errors occurred for %d packages", len(errors))
		return fmt.Errorf("errors occurred while executing command for packages: \n%s", errors.Error())
	}

	return nil
}

func validateSubCommand(subCommand string) error {
	if !slices.Contains(getAllowedSubCommands(), subCommand) {
		return fmt.Errorf("invalid subcommand: %s. Allowed subcommands are: [%s]", subCommand, strings.Join(getAllowedSubCommands(), ", "))
	}

	return nil
}
