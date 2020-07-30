package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func setupValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the integration",
		Long:  "Use validate command to validate the integration files.",
		RunE:  validateCommandAction,
	}
	return cmd
}

func validateCommandAction(cmd *cobra.Command, args []string) error {
	fmt.Println("validate is not implemented yet.")
	return nil
}
