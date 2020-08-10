package cmd

import (
	"os"

	"github.com/elastic/package-spec/code/go/pkg/validator"
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
	packageRootPath, err := os.Getwd()
	if err != nil {
		return err
	}

	return validator.ValidateFromPath(packageRootPath)
}
