// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/reporters/formats"
	"github.com/elastic/elastic-package/internal/testrunner/reporters/outputs"
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
	cmd.PersistentFlags().StringSliceP(cobraext.DataStreamsFlagName, "d", nil, cobraext.DataStreamsFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.ReportFormatFlagName, "", string(formats.ReportFormatHuman), cobraext.ReportFormatFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.ReportOutputFlagName, "", string(outputs.ReportOutputSTDOUT), cobraext.ReportOutputFlagDescription)

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

		dataStreams, err := cmd.Flags().GetStringSlice(cobraext.DataStreamsFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.DataStreamsFlagName)
		}

		generateTestResult, err := cmd.Flags().GetBool(cobraext.GenerateTestResultFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.GenerateTestResultFlagName)
		}

		reportFormat, err := cmd.Flags().GetString(cobraext.ReportFormatFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.ReportFormatFlagName)
		}

		reportOutput, err := cmd.Flags().GetString(cobraext.ReportOutputFlagName)
		if err != nil {
			return cobraext.FlagParsingError(err, cobraext.ReportOutputFlagName)
		}

		packageRootPath, found, err := packages.FindPackageRoot()
		if !found {
			return errors.New("package root not found")
		}
		if err != nil {
			return errors.Wrap(err, "locating package root failed")
		}

		testFolders, err := testrunner.FindTestFolders(packageRootPath, testType, dataStreams)
		if err != nil {
			return errors.Wrap(err, "unable to determine test folder paths")
		}

		if failOnMissing && len(testFolders) == 0 {
			if len(dataStreams) > 0 {
				return fmt.Errorf("no %s tests found for %s data stream(s)", testType, strings.Join(dataStreams, ","))
			}
			return fmt.Errorf("no %s tests found", testType)
		}

		esClient, err := elasticsearch.Client()
		if err != nil {
			return errors.Wrap(err, "fetching Elasticsearch client instance failed")
		}

		var results []testrunner.TestResult
		for _, folder := range testFolders {
			r, err := testrunner.Run(testType, testrunner.TestOptions{
				TestFolder:         folder,
				PackageRootPath:    packageRootPath,
				GenerateTestResult: generateTestResult,
				ESClient:           esClient,
			})

			results = append(results, r...)

			if err != nil {
				return errors.Wrapf(err, "error running package %s tests", testType)
			}
		}

		format := testrunner.TestReportFormat(reportFormat)
		report, err := testrunner.FormatReport(format, results)
		if err != nil {
			return errors.Wrap(err, "error formatting test report")
		}

		m, err := packages.ReadPackageManifest(filepath.Join(packageRootPath, packages.PackageManifestFile))
		if err != nil {
			return errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRootPath)
		}

		if err := testrunner.WriteReport(m.Name, testrunner.TestReportOutput(reportOutput), report, format); err != nil {
			return errors.Wrap(err, "error writing test report")
		}

		// Check if there is any error or failure reported
		for _, r := range results {
			if r.ErrorMsg != "" || r.FailureMsg != "" {
				return errors.New("one or more test cases failed")
			}
		}
		return nil
	}
}
