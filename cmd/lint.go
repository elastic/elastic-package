package cmd

import (
	"github.com/elastic/package-spec/code/go/pkg/validator"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
)

func setupLintCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint the package",
		Long:  "Use lint command to lint the package files.",
		RunE:  lintCommandAction,
	}
	return cmd
}

func lintCommandAction(cmd *cobra.Command, args []string) error {
	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return errors.New("package root not found")
	}
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}

	return validator.ValidateFromPath(packageRootPath)
}
