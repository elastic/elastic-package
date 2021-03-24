// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/spf13/cobra"
)

const createLongDescription = `Use this command to create a new package or add more data streams.

The command can help bootstrap the first draft of a package using embedded package template. It can be used to extend the package with more data streams.

Context:
  global, package`

const createPackageLongDescription = `Use this command to create a new package.

The command can bootstrap the first draft of a package using embedded package template and wizard.

Context:
  global`

const createDataStreamLongDescription = `Use this command to add a new data stream to the existing package.

The command can extend the package with a new data stream using a dedicated wizard.

Context:
  package`

func setupCreateCommand() *cobra.Command {
	createPackageCmd := &cobra.Command{
		Use:   "package",
		Short: "Create new package",
		Long:  createPackageLongDescription,
		RunE:  createPackageCommandAction,
	}

	createDataStreamCmd := &cobra.Command{
		Use:   "data-stream",
		Short: "Create new data stream",
		Long:  createDataStreamLongDescription,
		RunE:  createDataStreamCommandAction,
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create package resources",
		Long:  createLongDescription,
	}
	cmd.AddCommand(createPackageCmd)
	cmd.AddCommand(createDataStreamCmd)
	return cmd
}

func createPackageCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Create a new package")

	cmd.Println("Done")
	return nil
}

func createDataStreamCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Create a new data stream")

	cmd.Println("Done")
	return nil
}
