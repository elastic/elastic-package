package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/system"
)

func setupTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run test suite for the package",
		Long:  "Use test runners to verify if the package collects logs and metrics properly.",
		RunE:  testCommandAction,
	}
	cmd.PersistentFlags().BoolP("fail-on-missing", "m", false, "fail if tests are missing")

	testTypes := []testrunner.TestType{testrunner.TestTypeSystem}
	for _, testType := range testTypes {
		testTypeCmd := &cobra.Command{
			Use:   string(testType),
			Short: fmt.Sprintf("Run %s tests", testType),
			Long:  fmt.Sprintf("Run %s tests for a package", testType),
			RunE:  testTypeCommandActionFactory(testType),
		}

		cmd.AddCommand(testTypeCmd)
	}

	return cmd
}

func testTypeCommandActionFactory(testType testrunner.TestType) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		packageRootPath, found, err := packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return errors.Wrap(err, "locating package root failed")
		}

		// TODO: populate datasets argument
		testFolderPaths, err := testrunner.FindTestFolders(packageRootPath, testType, nil)
		if err != nil {
			return errors.Wrap(err, "unable to determine test folder paths")
		}

		// TODO: account for fail on missin
		failOnMissing, err := cmd.Flags().GetBool("fail-on-missing")
		if err != nil {
			return errors.Wrap(err, "error parsing --fail-on-missing flag")
		}

		if failOnMissing && len(testFolderPaths) == 0 {
			return fmt.Errorf("no %s tests found", testType)
		}

		for _, path := range testFolderPaths {
			if err := system.Run(path); err != nil {
				return errors.Wrap(err, "error running package system tests")
			}
		}

		return nil
	}
}

func testCommandAction(cmd *cobra.Command, args []string) error {
	// TODO: call actions for other types of tests
	return testTypeCommandActionFactory(testrunner.TestTypeSystem)(cmd, args)
}
