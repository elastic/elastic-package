// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
)

const publishLongDescription = `[DEPRECATED] Use this command to publish a new package revision.

The command checks if the package hasn't been already published (whether it's present in snapshot/staging/production branch or open as pull request). If the package revision hasn't been published, it will open a new pull request.`

func setupPublishCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:        "publish",
		Short:      "Publish the package to the Package Registry",
		Long:       publishLongDescription,
		Deprecated: "Package candidates to the Package Storage v2 are published using CI jobs. README: https://github.com/elastic/elastic-package/blob/main/docs/howto/use_package_storage_v2.md",
		RunE:       publishCommandAction,
	}

	// Fork flag can be a workaround for users that don't own forks of the Package Storage.
	cmd.Flags().BoolP(cobraext.ForkFlagName, "f", true, cobraext.ForkFlagDescription)

	// SkipPullRequest flag can used to verify if the "publish" command works properly (finds correct revisions),
	// for which the operator doesn't want to immediately close just opened PRs (standard dry-run).
	cmd.Flags().BoolP(cobraext.SkipPullRequestFlagName, "s", false, cobraext.SkipPullRequestFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func publishCommandAction(cmd *cobra.Command, args []string) error {
	return nil
}
