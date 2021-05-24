// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/github"
	"github.com/elastic/elastic-package/internal/promote"
	"github.com/elastic/elastic-package/internal/storage"
)

const promoteLongDescription = `Use this command to move packages between the snapshot, staging, and production stages of the package registry.

This command is intended primarily for use by administrators.

It allows for selecting packages for promotion and opens new pull requests to review changes. Please be aware that the tool checks out an in-memory Git repository and switches over branches (snapshot, staging and production), so it may take longer to promote a larger number of packages.`

func setupPromoteCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:          "promote",
		Short:        "Promote packages",
		Long:         promoteLongDescription,
		RunE:         promoteCommandAction,
		SilenceUsage: true,
	}

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func promoteCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Promote packages")

	// Setup GitHub
	err := github.EnsureAuthConfigured()
	if err != nil {
		return errors.Wrap(err, "GitHub auth configuration failed")
	}

	githubClient, err := github.Client()
	if err != nil {
		return errors.Wrap(err, "creating GitHub client failed")
	}

	githubUser, err := github.User(githubClient)
	if err != nil {
		return errors.Wrap(err, "fetching GitHub user failed")
	}
	cmd.Printf("Current GitHub user: %s\n", githubUser)

	// Prompt for promotion options
	sourceStage, destinationStage, err := promptPromotion()
	if err != nil {
		return errors.Wrap(err, "prompt for promotion failed")
	}

	newestOnly, err := promptPromoteNewestOnly()
	if err != nil {
		return errors.Wrap(err, "prompt for promoting newest versions only failed")
	}

	cmd.Println("Cloning repository...")
	repository, err := storage.CloneRepository(githubUser, sourceStage)
	if err != nil {
		return errors.Wrapf(err, "cloning source repository failed (branch: %s)", sourceStage)
	}

	cmd.Println("Creating list of packages...")
	allPackages, err := storage.ListPackages(repository)
	if err != nil {
		return errors.Wrapf(err, "listing packages failed")
	}

	packagesToBeSelected := allPackages.FilterPackages(newestOnly)
	if len(packagesToBeSelected) == 0 {
		fmt.Println("No packages available for promotion.")
		return nil
	}

	promotedPackages, err := promptPackages(packagesToBeSelected)
	if err != nil {
		return errors.Wrap(err, "prompt for package selection failed")
	}

	removedPackages := promote.DeterminePackagesToBeRemoved(allPackages, promotedPackages, newestOnly)

	nonce := time.Now().UnixNano()
	// Copy packages to destination
	fmt.Printf("Promote packages from %s to %s...\n", sourceStage, destinationStage)
	newDestinationBranch := fmt.Sprintf("promote-from-%s-to-%s-%d", sourceStage, destinationStage, nonce)
	err = storage.CopyPackages(repository, sourceStage, destinationStage, promotedPackages, newDestinationBranch)
	if err != nil {
		return errors.Wrapf(err, "copying packages failed (source: %s, destination: %s)", sourceStage, destinationStage)
	}

	// Remove packages from source
	newSourceBranch := fmt.Sprintf("delete-from-%s-%d", sourceStage, nonce)
	err = storage.RemovePackages(repository, sourceStage, removedPackages, newSourceBranch)
	if err != nil {
		return errors.Wrapf(err, "removing packages failed (source: %s)", sourceStage)
	}

	// Push changes
	err = storage.PushChanges(githubUser, repository, newSourceBranch, newDestinationBranch)
	if err != nil {
		return errors.Wrapf(err, "pushing changes failed")
	}

	// Calculate package signatures
	signedPackages, err := storage.CalculatePackageSignatures(repository, newDestinationBranch, promotedPackages)
	if err != nil {
		return errors.Wrap(err, "signing packages failed")
	}

	// Open PRs
	url, err := promote.OpenPullRequestWithPromotedPackages(githubClient, githubUser, newDestinationBranch, destinationStage, sourceStage, destinationStage, signedPackages)
	if err != nil {
		return errors.Wrapf(err, "opening PR with promoted packages failed (head: %s, base: %s)", newDestinationBranch, destinationStage)
	}
	cmd.Println("Pull request with promoted packages:", url)

	url, err = promote.OpenPullRequestWithRemovedPackages(githubClient, githubUser, newSourceBranch, sourceStage, sourceStage, url, removedPackages)
	if err != nil {
		return errors.Wrapf(err, "opening PR with removed packages failed (head: %s, base: %s)", newDestinationBranch, destinationStage)
	}
	cmd.Println("Pull request with removed packages:", url)

	cmd.Println("Done")
	return nil
}

func promptPromotion() (string, string, error) {
	promotionPrompt := &survey.Select{
		Message: "Which promotion would you like to run",
		Options: []string{"snapshot - staging", "staging - production", "snapshot - production"},
		Default: "snapshot - staging",
	}

	var promotion string
	err := survey.AskOne(promotionPrompt, &promotion)
	if err != nil {
		return "", "", err
	}

	s := strings.Split(promotion, " - ")
	return s[0], s[1], nil
}

func promptPromoteNewestOnly() (bool, error) {
	newestOnly := true
	prompt := &survey.Confirm{
		Message: "Would you like to promote newest versions only and remove older ones?",
		Default: true,
	}
	err := survey.AskOne(prompt, &newestOnly)
	if err != nil {
		return false, err
	}
	return newestOnly, nil
}

func promptPackages(packages storage.PackageVersions) (storage.PackageVersions, error) {
	packagesPrompt := &survey.MultiSelect{
		Message:  "Which packages would you like to promote",
		Options:  packages.Strings(),
		PageSize: 100,
	}

	var selectedOptions []string
	err := survey.AskOne(packagesPrompt, &selectedOptions, survey.WithValidator(survey.Required))
	if err != nil {
		return nil, err
	}

	var selected storage.PackageVersions
	for _, option := range selectedOptions {
		for _, p := range packages {
			if p.String() == option {
				selected = append(selected, p)
			}
		}
	}
	return selected, nil
}
