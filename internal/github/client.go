// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v32/github"
)

type Client struct {
	client *github.Client
}

// UnauthorizedClient function returns unauthorized instance of Github API client.
func UnauthorizedClient() *Client {
	githubClient := github.NewClient(new(http.Client))
	return &Client{githubClient}
}

func (c *Client) LatestRelease(ctx context.Context, repositoryOwner, repositoryName string) (*github.RepositoryRelease, error) {
	release, _, err := c.client.Repositories.GetLatestRelease(ctx, repositoryOwner, repositoryName)
	if err != nil {
		return nil, fmt.Errorf("can't check latest release: %w", err)
	}

	if release.TagName == nil || *release.TagName == "" {
		return nil, fmt.Errorf("release tag is empty")
	}

	return release, nil
}
