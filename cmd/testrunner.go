// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
	_ "github.com/elastic/elastic-package/internal/testrunner/runners" // register all test runners
)

func setupTestCommand() *cobra.Command {
	var testTypeCmdActions []cobraext.CommandAction

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run test suite for the package",
		Long:  "Use test runners to verify if the package collects logs and metrics properly.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Run test suite for the package")

			if len(args) > 0 {
				return fmt.Errorf("unsupported test type: %s", args[0])
			}

			return cobraext.ComposeCommandActions(cmd, args, testTypeCmdActions...)
		}}

	cmd.PersistentFlags().BoolP(cobraext.FailOnMissingFlagName, "m", false, cobraext.FailOnMissingFlagDescription)
	cmd.PersistentFlags().BoolP(cobraext.GenerateTestResultFlagName, "g", false, cobraext.GenerateTestResultFlagDescription)
	cmd.PersistentFlags().StringSliceP(cobraext.DatasetsFlagName, "d", nil, cobraext.DatasetsFlagDescription)

	for _, testType := range testrunner.TestTypes() {
		action := testTypeCommandActionFactory(testType)
		testTypeCmdActions = append(testTypeCmdActions, action)

		testTypeCmd := &cobra.Command{
			Use:   string(testType),
			Short: fmt.Sprintf("Run %s tests", testType),
			Long:  fmt.Sprintf("Run %s tests for the package", testType),
			RunE:  action,
		}

		cmd.AddCommand(testTypeCmd)
	}

	return cmd
}

func testTypeCommandActionFactory(testType testrunner.TestType) cobraext.CommandAction {
	return func(cmd *cobra.Command, args []string) error {
		cmd.Printf("Run %s tests for the package\n", testType)

		failOnMissing, err := cmd.Flags().GetBool(cobraext.FailOnMissingFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.FailOnMissingFlagName)
		}

		datasets, err := cmd.Flags().GetStringSlice(cobraext.DatasetsFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.DatasetsFlagName)
		}

		generateTestResult, err := cmd.Flags().GetBool(cobraext.GenerateTestResultFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.GenerateTestResultFlagName)
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
			if len(datasets) > 0 {
				return fmt.Errorf("no %s tests found for %s dataset(s)", testType, strings.Join(datasets, ","))
			}
			return fmt.Errorf("no %s tests found", testType)
		}

		for _, path := range testFolderPaths {
			if err := testrunner.Run(testType, testrunner.TestOptions{
				TestFolderPath:     path,
				PackageRootPath:    packageRootPath,
				GenerateTestResult: generateTestResult,
			}); err != nil {
				return errors.Wrapf(err, "error running package %s tests", testType)
			}
		}

		cmd.Println("Done")
		return nil
	}
}
