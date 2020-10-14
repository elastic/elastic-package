// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package promote

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v32/github"
	"github.com/pkg/errors"
)

const (
	repositoryOwner = "elastic"
	repositoryName  = "package-storage"
)

// OpenPullRequestWithRemovedPackages method opens a PR against "base" branch with removed packages.
// Head is the branch containing the changes that will be added to the base branch.
func OpenPullRequestWithRemovedPackages(client *github.Client, username, head, base, sourceStage, promotionURL string, promotedPackages PackageVersions) (string, error) {
	title := fmt.Sprintf("[%s] Remove packages from %s", sourceStage, sourceStage)
	description := buildPullRequestRemoveDescription(sourceStage, promotionURL, promotedPackages)
	return openPullRequestWithPackages(client, username, head, base, title, description)
}

// OpenPullRequestWithPromotedPackages method opens a PR against "base" branch with promoted packages.
// Head is the branch containing the changes that will be added to the base branch.
func OpenPullRequestWithPromotedPackages(client *github.Client, username, head, base, sourceStage, destinationStage string, promotedPackages PackageVersions, r *git.Repository) (string, error) {
	title := fmt.Sprintf("[%s] Promote packages from %s to %s", destinationStage, sourceStage, destinationStage)
	description, err := buildPullRequestPromoteDescription(sourceStage, destinationStage, promotedPackages, r, head)
	if err != nil {
		return "", err
	}
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

func buildPullRequestRemoveDescription(sourceStage, promotionURL string, versions PackageVersions) string {
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

func buildPullRequestPromoteDescription(sourceStage, destinationStage string, versions PackageVersions, r *git.Repository, head string) (string, error) {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("This PR promotes packages from `%s` to `%s`.\n", sourceStage, destinationStage))
	builder.WriteString("\n")
	builder.WriteString("Promoted packages:\n")

	for _, packageVersion := range versions {
		sig, err := calculatePackageSignature(r, head, packageVersion)
		if err != nil {
			return "", err
		}
		builder.WriteString(fmt.Sprintf("* `%s: %s`\n", packageVersion.String(), sig))
	}
	return builder.String(), nil
}

// calculatePackageSignature computes the combined sha1 hash for all the files in the package
// this is equivalent to doing find <package> -type f -exec <hash tool> {} + | awk '{print $1}' | sort | <hash tool>
// on the package version directory
func calculatePackageSignature(r *git.Repository, branch string, packageVersion PackageVersion) (string, error) {
	wt, err := r.Worktree()
	if err != nil {
		return "", errors.Wrap(err, "fetching worktree reference failed while calculating directory hash")
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
	})
	if err != nil {
		return "", errors.Wrapf(err, "changing branch failed (path: %s) while calculating directory hash", branch)
	}
	resources, err := walkPackageResources(wt.Filesystem, packageVersion.path())
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve package paths while calculating directory hash")
	}

	return calculateFilesSignature(wt.Filesystem, resources)
}
