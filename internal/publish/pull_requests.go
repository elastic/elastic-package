// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package publish

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

const (
	repositoryName  = "package-storage"
	repositoryOwner = "elastic"

	defaultPackageOwnerTeam = "elastic/integrations"
)

func checkIfPullRequestAlreadyOpen(githubClient *github.Client, manifest packages.PackageManifest) (bool, error) {
	expectedTitle := buildPullRequestTitle(manifest)
	q := fmt.Sprintf(`repo:elastic/package-storage base:snapshot is:pr is:open in:title "%s"`, expectedTitle)
	logger.Debugf("Use Search API to find an open pull request (query: '%s')", q)
	searchResults, _, err := githubClient.Search.Issues(context.Background(), q, new(github.SearchOptions))
	if err != nil {
		return false, errors.Wrap(err, "can't search pull requests")
	}

	for _, item := range searchResults.Issues {
		if *item.Title == expectedTitle {
			logger.Debugf("Found pull request: %s", *item.HTMLURL)
			return true, nil
		}
	}
	return false, nil
}

func openPullRequest(githubClient *github.Client, githubUser, destinationBranch string, manifest packages.PackageManifest, commitHash string, fork bool) error {
	user := repositoryOwner
	if fork {
		user = githubUser
	}
	logger.Debugf("Current user: %s", user)

	title := buildPullRequestTitle(manifest)
	diffURL := buildPullRequestDiffURL(user, commitHash)
	description := buildPullRequestDescription(manifest, diffURL)

	userHead := fmt.Sprintf("%s:%s", user, destinationBranch)
	maintainerCanModify := true
	base := snapshotStage

	logger.Debugf("Create new pull request (head: %s, base: %s)", userHead, base)
	pullRequest, _, err := githubClient.PullRequests.Create(context.Background(), repositoryOwner, repositoryName, &github.NewPullRequest{
		Title:               &title,
		Head:                &userHead,
		Base:                &base,
		Body:                &description,
		MaintainerCanModify: &maintainerCanModify,
	})
	if err != nil {
		return errors.Wrap(err, "can't open new pull request")
	}

	logger.Debugf("Pull request URL: %s", *pullRequest.HTMLURL)

	// Try to set reviewers
	reviewers := buildReviewersRequest(manifest)
	logger.Debugf("Update reviewers (pull request ID: %d)", *pullRequest.Number)
	pullRequest, _, err = githubClient.PullRequests.RequestReviewers(context.Background(), repositoryOwner, repositoryName, *pullRequest.Number, reviewers)
	if err != nil {
		return errors.Wrap(err, "can't request reviewers, please double-check if the owner exists and has access to the repository")
	}

	if len(pullRequest.RequestedTeams) != 0 || len(pullRequest.RequestedReviewers) != 0 {
		logger.Debugf("Reviewers requested successfully (teams: %d, reviewers: %d)", len(pullRequest.RequestedTeams), len(pullRequest.RequestedReviewers))
		return nil
	}

	// Fallback reviewers to default package owner
	logger.Debugf("Update reviewers with default owner (pull request ID: %d)", *pullRequest.Number)
	_, _, err = githubClient.PullRequests.RequestReviewers(context.Background(), repositoryOwner, repositoryName, *pullRequest.Number,
		buildDefaultReviewersRequest())
	if err != nil {
		return errors.Wrap(err, "can't request reviewers with default owner")
	}
	return nil
}

func buildPullRequestTitle(manifest packages.PackageManifest) string {
	return fmt.Sprintf(`[snapshot] Update "%s" package to version %s`, manifest.Name, manifest.Version)
}

func buildPullRequestDiffURL(username, commitHash string) string {
	return fmt.Sprintf("https://github.com/%s/package-storage/commit/%s", username, commitHash)
}

func buildPullRequestDescription(manifest packages.PackageManifest, diffURL string) string {
	return fmt.Sprintf("This PR updates `%s` package to version %s.\n\nChanges: %s", manifest.Name,
		manifest.Version, diffURL)
}

func buildReviewersRequest(manifest packages.PackageManifest) github.ReviewersRequest {
	if manifest.Owner.Github == "" {
		return buildDefaultReviewersRequest()
	}

	if i := strings.Index(manifest.Owner.Github, "/"); i > -1 {
		return github.ReviewersRequest{TeamReviewers: []string{manifest.Owner.Github[i+1:]}}
	}
	return github.ReviewersRequest{Reviewers: []string{manifest.Owner.Github}}
}

func buildDefaultReviewersRequest() github.ReviewersRequest {
	return github.ReviewersRequest{TeamReviewers: []string{defaultPackageOwnerTeam}}
}
