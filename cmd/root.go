package cmd

import (
	"github.com/spf13/cobra"
)

// RootCmd creates and returns root cmd for elastic-package
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "elastic-package",
		Short: "elastic-package - Command line tool for developing Elastic Integrations",
	}
	rootCmd.AddCommand(
		setupCheckCommand(),
		setupClusterCommand(),
		setupBuildCommand(),
		setupFormatCommand(),
		setupTestCommand(),
		setupValidateCommand())

	return rootCmd
}
