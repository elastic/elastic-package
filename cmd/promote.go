// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
)

const promoteLongDescription = `[DEPRECATED] Use this command to move packages between the snapshot, staging, and production stages of the package registry.

This command is intended primarily for use by administrators.

It allows for selecting packages for promotion and opens new pull requests to review changes. Please be aware that the tool checks out an in-memory Git repository and switches over branches (snapshot, staging and production), so it may take longer to promote a larger number of packages.`

func setupPromoteCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:          "promote",
		Short:        "Promote packages",
		Long:         promoteLongDescription,
		RunE:         promoteCommandAction,
		Deprecated:   "Packages stored in the Package Storage v2 do not require to be promoted. README: https://github.com/elastic/elastic-package/blob/main/docs/howto/use_package_storage_v2.md",
		SilenceUsage: true,
	}

	cmd.Flags().StringP(cobraext.DirectionFlagName, "d", "", cobraext.DirectionFlagDescription)
	cmd.Flags().BoolP(cobraext.NewestOnlyFlagName, "n", false, cobraext.NewestOnlyFlagDescription)
	cmd.Flags().StringSliceP(cobraext.PromotedPackagesFlagName, "p", nil, cobraext.PromotedPackagesFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func promoteCommandAction(cmd *cobra.Command, _ []string) error {
	return nil
}
