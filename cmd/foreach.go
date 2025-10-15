package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/filter"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

const foreachLongDescription = `Execute a command for each package matching the given filter criteria.

This command combines filtering capabilities with command execution, allowing you to run
any elastic-package subcommand across multiple packages in a single operation.

The command uses the same filter flags as the 'filter' command (--input, --code-owner, 
--kibana-version, --category) to select packages, then executes the specified subcommand
for each matched package.`

func setupForeachCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "foreach [filter-flags] --exec <subcommand> [subcommand-flags]",
		Short: "Execute a command for filtered packages",
		Long:  foreachLongDescription,
		Example: `  # Run system tests for packages with specific inputs
  elastic-package foreach --input tcp,udp --exec test system -g`,
		RunE: foreachCommandAction,
	}

	filter.SetFilterFlags(cmd)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func foreachCommandAction(cmd *cobra.Command, args []string) error {
	// Find integration root
	root, err := packages.MustFindIntegrationRoot()
	if err != nil {
		return fmt.Errorf("can't find integration root: %w", err)
	}

	// reuse filterPackage from cmd/filter.go
	filtered, err := filterPackage(cmd)
	if err != nil {
		return fmt.Errorf("filtering packages failed: %w", err)
	}
	fmt.Printf("Found %d matching package(s)\n", len(filtered))

	// Execute command for each package
	for _, pkg := range filtered {
		// Get elastic-package command
		ep := cmd.Parent()

		// Set change directory flag to the package directory
		ep.Flags().Set(cobraext.ChangeDirectoryFlagName, filepath.Join(root, "packages", pkg.Name))

		ep.SetArgs(args)

		// Execute command
		if err := ep.Execute(); err != nil {
			return fmt.Errorf("executing command for package %s failed: %w", pkg.Name, err)
		}
	}

	return nil
}
