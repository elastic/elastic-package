package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func setupLintCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint the integration",
		Long:  "Use lint command to lint the integration files.",
		RunE:  lintCommandAction,
	}
	return cmd
}

func lintCommandAction(cmd *cobra.Command, args []string) error {
	fmt.Println("lint is not implemented yet.")
	return nil
}
