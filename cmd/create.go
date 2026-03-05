// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
)

const createLongDescription = `Use this command to create a new package or add more data streams.

The command can help bootstrap the first draft of a package using embedded package template. It can be used to extend the package with more data streams.

For details on how to create a new package, review the [HOWTO guide](https://github.com/elastic/elastic-package/blob/main/docs/howto/create_new_package.md).`

// Flags for non-interactive mode
const (
	createPackageNonInteractiveFlag    = "non-interactive"
	createPackageTypeFlag              = "type"
	createPackageNameFlag              = "name"
	createDataStreamNonInteractiveFlag = "non-interactive"
	createDataStreamNameFlag           = "name"
	createDataStreamTypeFlag           = "type"
	createDataStreamInputsFlag         = "inputs"
)

func setupCreateCommand() *cobraext.Command {
	createPackageCmd := &cobra.Command{
		Use:   "package",
		Short: "Create new package",
		Long:  createPackageLongDescription,
		Args:  cobra.NoArgs,
		RunE:  createPackageCommandAction,
	}
	createPackageCmd.Flags().Bool(createPackageNonInteractiveFlag, false, "skip TUI wizard; requires --type and --name")
	createPackageCmd.Flags().String(createPackageTypeFlag, "integration", "package type (input, integration, content); required with --non-interactive")
	createPackageCmd.Flags().String(createPackageNameFlag, "new_package", "package name; required with --non-interactive")

	createDataStreamCmd := &cobra.Command{
		Use:   "data-stream",
		Short: "Create new data stream",
		Long:  createDataStreamLongDescription,
		Args:  cobra.NoArgs,
		RunE:  createDataStreamCommandAction,
	}
	createDataStreamCmd.Flags().Bool(createDataStreamNonInteractiveFlag, false, "skip TUI wizard; requires --name and --type; --inputs required when type is logs")
	createDataStreamCmd.Flags().String(createDataStreamNameFlag, "new_data_stream", "data stream name; required with --non-interactive")
	createDataStreamCmd.Flags().String(createDataStreamTypeFlag, "logs", "data stream type (logs, metrics); required with --non-interactive")
	createDataStreamCmd.Flags().StringSlice(createDataStreamInputsFlag, nil, "input types for logs data streams; required with --non-interactive when type is logs (e.g. filestream,tcp)")

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create package resources",
		Long:  createLongDescription,
	}
	cmd.AddCommand(createPackageCmd)
	cmd.AddCommand(createDataStreamCmd)

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}
