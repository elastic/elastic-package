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

func setupCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create package resources",
		Long:  createLongDescription,
		RunE:  createCommandAction,
	}
	return cmd
}

func createCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Create package resources")

	cmd.Println("Done")
	return nil
}
