// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/version"
)

const versionLongDescription = `Use this command to print the version of elastic-package that you have installed. This is especially useful when reporting bugs.`

func setupVersionCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Long:  versionLongDescription,
		RunE:  versionCommandAction,
	}

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func versionCommandAction(cmd *cobra.Command, args []string) error {
	var sb strings.Builder
	sb.WriteString("elastic-package ")
	if version.Tag != "" {
		sb.WriteString(version.Tag)
		sb.WriteString(" ")
	}
	sb.WriteString(fmt.Sprintf("version-hash %s (build time: %s)", version.CommitHash, version.BuildTimeFormatted()))
	fmt.Println(sb.String())
	return nil
}
