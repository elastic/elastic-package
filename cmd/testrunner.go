package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/system"
)

func setupTestCommand() *cobra.Command {
	// TODO: add more test types as their runners are implemented
	testTypes := []testrunner.TestType{testrunner.TestTypeSystem}
	var testTypeCmdActions []cobraext.CommandAction

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run test suite for the package",
		Long:  "Use test runners to verify if the package collects logs and metrics properly.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cobraext.ComposeCommandActions(cmd, args, testTypeCmdActions...)
		}}

	cmd.PersistentFlags().BoolP(cobraext.FailOnMissingFlagName, "m", false, cobraext.FailOnMissingFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.DatasetFlagName, "d", "", cobraext.DatasetFlagDescription)

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

func testTypeCommandActionFactory(testType testrunner.TestType) cobraext.CommandAction {
	return func(cmd *cobra.Command, args []string) error {
		failOnMissing, err := cmd.Flags().GetBool(cobraext.FailOnMissingFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.FailOnMissingFlagName)
		}

		dataset, err := cmd.Flags().GetString(cobraext.DatasetFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.DatasetFlagName)
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
