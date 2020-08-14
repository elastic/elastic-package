package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/system"
)

func setupTestCommand() *cobra.Command {
	// TODO: add more test types as their runners are implemented
	testTypes := []testrunner.TestType{testrunner.TestTypeSystem}
	var testTypeCmdActions []commandAction

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run test suite for the package",
		Long:  "Use test runners to verify if the package collects logs and metrics properly.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return composeCommandActions(cmd, args, testTypeCmdActions...)
		}}

	cmd.PersistentFlags().BoolP("fail-on-missing", "m", false, "fail if tests are missing")
	cmd.PersistentFlags().StringP("dataset", "d", "", "comma-separated datasets to test")

	for _, testType := range testTypes {
		action := testTypeCommandActionFactory(testType)
		testTypeCmdActions = append(testTypeCmdActions, action)

		testTypeCmd := &cobra.Command{
			Use:   string(testType),
			Short: fmt.Sprintf("Run %s tests", testType),
			Long:  fmt.Sprintf("Run %s tests for a package", testType),
			RunE:  action,
		}

		cmd.AddCommand(testTypeCmd)
	}

	return cmd
}

func testTypeCommandActionFactory(testType testrunner.TestType) commandAction {
	return func(cmd *cobra.Command, args []string) error {
		failOnMissing, err := cmd.Flags().GetBool("fail-on-missing")
		if err != nil {
			return errors.Wrap(err, "error parsing --fail-on-missing flag")
		}

		dataset, err := cmd.Flags().GetString("dataset")
		if err != nil {
			return errors.Wrap(err, "error parsing --dataset flag")
		}
		var datasets []string
		if dataset != "" {
			datasets = strings.Split(dataset, ",")
		}

		packageRootPath, found, err := packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return errors.Wrap(err, "locating package root failed")
		}

		testFolderPaths, err := testrunner.FindTestFolders(packageRootPath, testType, datasets)
		if err != nil {
			return errors.Wrap(err, "unable to determine test folder paths")
		}

		if failOnMissing && len(testFolderPaths) == 0 {
			return fmt.Errorf("no %s tests found for %s dataset(s)", testType, dataset)
		}

		for _, path := range testFolderPaths {
			if err := system.Run(path); err != nil {
				return errors.Wrap(err, "error running package system tests")
			}
		}

		return nil
	}
}
