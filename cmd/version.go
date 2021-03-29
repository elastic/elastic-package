// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/version"
)

func init() {
	cobraext.CommandInfos[versionCmd] = cobraext.CommandInfo{
		Short:   "Show application version",
		Long:    versionLongDescription,
		Context: "global",
	}
}

const versionCmd = "version"
const versionLongDescription = `Use this command to print the version of elastic-package that you have installed. This is especially useful when reporting bugs.`

func setupVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   versionCmd,
		Short: cobraext.CommandInfos[versionCmd].Short,
		Long:  cobraext.CommandInfos[versionCmd].LongCLI(),
		RunE:  versionCommandAction,
	}
	return cmd
}

func versionCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Printf("elastic-package version-hash %s (build time: %s)\n", version.CommitHash, version.BuildTimeFormatted())
	return nil
}
