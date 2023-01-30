// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
)

const generateLongDescription = `Use this command to generate benchmarks data for a package. Currently, only data for what we have related assets on https://github.com/elastic/elastic-integration-corpus-generator-tool are supported.`

func setupGenerateCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate benchmarks data for the package",
		Long:  generateLongDescription,
		RunE:  generateDataStreamCommandAction,
	}

	cmd.PersistentFlags().StringP(cobraext.PackageFlagName, cobraext.PackageFlagShorthand, "", cobraext.PackageFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.GenerateDataStreamFlagName, cobraext.GenerateDataStreamFlagShorthand, "", cobraext.GenerateDataStreamFlagDescription)
	cmd.PersistentFlags().StringP(cobraext.GenerateSizeFlagName, cobraext.GenerateSizeFlagShorthand, "", cobraext.GenerateSizeFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}
