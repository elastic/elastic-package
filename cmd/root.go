package cmd

import (
	"github.com/spf13/cobra"
)

// RootCmd creates and returns root cmd for elastic-package
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "elastic-package",
		Short:        "elastic-package - Command line tool for developing Elastic Integrations",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(
		setupBuildCommand(),
		setupCheckCommand(),
		setupClusterCommand(),
		setupFormatCommand(),
		setupLintCommand(),
		setupPromoteCommand(),
		setupTestCommand(),
		setupVersionCommand())
	return rootCmd
}
