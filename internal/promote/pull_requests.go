// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package promote

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/storage"
)

const (
	repositoryOwner = "elastic"
	repositoryName  = "package-storage"

	githubTitleCharacterLimit = 256
)

// OpenPullRequestWithRemovedPackages method opens a PR against "base" branch with removed packages.
// Head is the branch containing the changes that will be added to the base branch.
func OpenPullRequestWithRemovedPackages(client *github.Client, username, head, base, sourceStage, promotionURL string, removedPackages storage.PackageVersions) (string, error) {
	title := buildPullRequestRemoveTitle(sourceStage, removedPackages)
	description := buildPullRequestRemoveDescription(sourceStage, promotionURL, removedPackages)
	return openPullRequestWithPackages(client, username, head, base, title, description)
}

// OpenPullRequestWithPromotedPackages method opens a PR against "base" branch with promoted packages.
// Head is the branch containing the changes that will be added to the base branch.
func OpenPullRequestWithPromotedPackages(client *github.Client, username, head, base, sourceStage, destinationStage string, signedPackages storage.SignedPackageVersions) (string, error) {
	title := buildPullRequestPromoteTitle(sourceStage, destinationStage, signedPackages.ToPackageVersions())
	description := buildPullRequestPromoteDescription(sourceStage, destinationStage, signedPackages)
	return openPullRequestWithPackages(client, username, head, base, title, description)
}

func openPullRequestWithPackages(client *github.Client, user, head, base, title, description string) (string, error) {
	userHead := fmt.Sprintf("%s:%s", user, head)
	maintainerCanModify := true
	pullRequest, _, err := client.PullRequests.Create(context.Background(), repositoryOwner, repositoryName, &github.NewPullRequest{
		Title:               &title,
		Head:                &userHead,
		Base:                &base,
		Body:                &description,
		MaintainerCanModify: &maintainerCanModify,
	})
	if err != nil {
		return "", errors.Wrap(err, "opening pull request failed")
	}

	_, _, err = client.Issues.Edit(context.Background(), repositoryOwner, repositoryName, *pullRequest.Number, &github.IssueRequest{
		Assignees: &[]string{
			user,
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "editing assignees in the pull request failed")
	}
	return *pullRequest.HTMLURL, nil
}

func buildPullRequestPromoteTitle(sourceStage, destinationStage string, promotedPackages storage.PackageVersions) string {
	details := strings.Join(promotedPackages.Strings(), ", ")
	title := fmt.Sprintf("[%s] Promote packages from %s (%s)", destinationStage, sourceStage, details)
	if len(title) > githubTitleCharacterLimit {
		return fmt.Sprintf("[%s] Promote many packages from %s", destinationStage, sourceStage)
	}
	return title
}

func buildPullRequestPromoteDescription(sourceStage, destinationStage string, signedPackages storage.SignedPackageVersions) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("This PR promotes packages from `%s` to `%s`.\n", sourceStage, destinationStage))
	builder.WriteString("\n")
	builder.WriteString("Promoted packages:\n")
	for _, str := range signedPackages.Strings() {
		builder.WriteString(fmt.Sprintf("* `%s`\n", str))
	}
	return builder.String()
}

func buildPullRequestRemoveTitle(stage string, removedPackages storage.PackageVersions) string {
	details := strings.Join(removedPackages.Strings(), ", ")
	title := fmt.Sprintf("[%s] Remove promoted packages (%s)", stage, details)
	if len(title) > githubTitleCharacterLimit {
		return fmt.Sprintf("[%s] Remove many promoted packages", stage)
	}
	return title
}

func buildPullRequestRemoveDescription(sourceStage, promotionURL string, versions storage.PackageVersions) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("This PR removes packages from `%s`.\n", sourceStage))
	builder.WriteString("\n")
	builder.WriteString("Removed packages:\n")
	for _, str := range versions.Strings() {
		builder.WriteString(fmt.Sprintf("* `%s`\n", str))
	}
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("Please make sure that the promotion PR is merged first: %s", promotionURL))
	return builder.String()
}
