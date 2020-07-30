package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func setupFormatCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Format the integration",
		Long:  "Use format command to format the integration files.",
		RunE:  formatCommandAction,
	}
	return cmd
}

func formatCommandAction(cmd *cobra.Command, args []string) error {
	fmt.Println("format is not implemented yet.")
	return nil
}
