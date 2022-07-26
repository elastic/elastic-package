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

func setupCreateCommand() *cobraext.Command {
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

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}
