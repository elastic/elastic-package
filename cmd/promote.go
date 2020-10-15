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

	"github.com/elastic/elastic-package/internal/github"
	"github.com/elastic/elastic-package/internal/promote"
)

func setupPromoteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "promote",
		Short:        "Promote packages",
		Long:         "Use promote command to move packages between stages in package-storage.",
		RunE:         promoteCommandAction,
		SilenceUsage: true,
	}
	return cmd
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
	repository, err := promote.CloneRepository(githubUser, sourceStage)
	if err != nil {
		return errors.Wrapf(err, "cloning source repository failed (branch: %s)", sourceStage)
	}

	cmd.Println("Creating list of packages...")
	allPackages, err := promote.ListPackages(repository)
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
	newDestinationStage, err := promote.CopyPackages(repository, sourceStage, destinationStage, promotedPackages, nonce)
	if err != nil {
		return errors.Wrapf(err, "copying packages failed (source: %s, destination: %s)", sourceStage, destinationStage)
	}

	// Remove packages from source
	newSourceStage, err := promote.RemovePackages(repository, sourceStage, removedPackages, nonce)
	if err != nil {
		return errors.Wrapf(err, "removing packages failed (source: %s)", sourceStage)
	}

	// Push changes
	err = promote.PushChanges(githubUser, repository, newSourceStage, newDestinationStage)
	if err != nil {
		return errors.Wrapf(err, "pushing changes failed")
	}

	// Calculate package signatures
	signedPackages, err := promote.CalculatePackageSignatures(repository, newDestinationStage, promotedPackages)
	if err != nil {
		return errors.Wrap(err, "signing packages failed")
	}

	// Open PRs
	url, err := promote.OpenPullRequestWithPromotedPackages(githubClient, githubUser, newDestinationStage, destinationStage, sourceStage, destinationStage, signedPackages)
	if err != nil {
		return errors.Wrapf(err, "opening PR with promoted packages failed (head: %s, base: %s)", newDestinationStage, destinationStage)
	}
	cmd.Println("Pull request with promoted packages:", url)

	url, err = promote.OpenPullRequestWithRemovedPackages(githubClient, githubUser, newSourceStage, sourceStage, sourceStage, url, removedPackages)
	if err != nil {
		return errors.Wrapf(err, "opening PR with removed packages failed (head: %s, base: %s)", newDestinationStage, destinationStage)
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

func promptPackages(packages promote.PackageVersions) (promote.PackageVersions, error) {
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

	var selected promote.PackageVersions
	for _, option := range selectedOptions {
		for _, p := range packages {
			if p.String() == option {
				selected = append(selected, p)
			}
		}
	}
	return selected, nil
}
