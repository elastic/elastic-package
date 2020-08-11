package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner/system"
)

func setupTestCommand() *cobra.Command {
	testSystemCmd := &cobra.Command{
		Use:   "system",
		Short: "Run system tests",
		Long:  "Run system tests for a package.",
		RunE:  testSystemCommandAction,
	}

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run test suite for the package",
		Long:  "Use test runners to verify if the package collects logs and metrics properly.",
		RunE:  testCommandAction,
	}
	cmd.AddCommand(
		testSystemCmd)
	return cmd
}

func testSystemCommandAction(cmd *cobra.Command, args []string) error {
	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return errors.New("package root not found")
	}
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}

	if err := system.Run(packageRootPath); err != nil {
		return errors.Wrap(err, "error running package system tests")
	}

	return nil
}

func testCommandAction(cmd *cobra.Command, args []string) error {
	return testSystemCommandAction(cmd, args)
}
