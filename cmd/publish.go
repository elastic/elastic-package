// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/github"
	"github.com/elastic/elastic-package/internal/publish"
)

const publishLongDescription = `Use this command to publish a new package revision.

The command checks if the package hasn't been already published (whether it's present in snapshot/staging/production branch or open as pull request). If the package revision hasn't been published, it will open a new pull request.`

func setupPublishCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish the package to the Package Registry",
		Long:  publishLongDescription,
		RunE:  publishCommandAction,
	}

	// Fork flag can be a workaround for users that don't own forks of the Package Storage.
	cmd.Flags().BoolP(cobraext.ForkFlagName, "f", true, cobraext.ForkFlagDescription)

	// SkipPullRequest flag can used to verify if the "publish" command works properly (finds correct revisions),
	// for which the operator doesn't want to immediately close just opened PRs (standard dry-run).
	cmd.Flags().BoolP(cobraext.SkipPullRequestFlagName, "s", false, cobraext.SkipPullRequestFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func publishCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Publish the package")

	fork, err := cmd.Flags().GetBool(cobraext.ForkFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ForkFlagName)
	}

	skipPullRequest, err := cmd.Flags().GetBool(cobraext.SkipPullRequestFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.SkipPullRequestFlagName)
	}

	// Setup GitHub
	err = github.EnsureAuthConfigured()
	if err != nil {
		return errors.Wrap(err, "GitHub auth configuration failed")
	}

	githubClient, err := github.Client()
	if err != nil {
		return errors.Wrap(err, "creating GitHub client failed")
	}

	// GitHub user
	githubUser, err := github.User(githubClient)
	if err != nil {
		return errors.Wrap(err, "fetching GitHub user failed")
	}
	cmd.Printf("Current GitHub user: %s\n", githubUser)

	// Publish the package
	err = publish.Package(githubUser, githubClient, fork, skipPullRequest)
	if err != nil {
		return errors.Wrap(err, "can't publish the package")
	}

	cmd.Println("Done")
	return nil
}
