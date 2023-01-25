// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

// Client function creates new instance of the GitHub API client.
func Client() (*github.Client, error) {
	authToken, err := AuthToken()
	if err != nil {
		return nil, fmt.Errorf("reading auth token failed: %s", err)
	}
	return github.NewClient(oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: authToken},
	))), nil
}

// UnauthorizedClient function returns unauthorized instance of Github API client.
func UnauthorizedClient() *github.Client {
	return github.NewClient(new(http.Client))
}

// User method returns the GitHub authenticated user.
func User(client *github.Client) (string, error) {
	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return "", fmt.Errorf("fetching authenticated user failed: %s", err)
	}
	return *user.Login, nil
}
