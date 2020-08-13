package github

import (
	"context"
	"github.com/pkg/errors"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

// Client method creates new instance of the GitHub API client.
func Client() (*github.Client, error) {
	authToken, err := AuthToken()
	if err != nil {
		return nil, errors.Wrap(err, "reading auth token failed")
	}
	return github.NewClient(oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: authToken},
	))), nil
}

// User method returns the GitHub authenticated user.
func User(client *github.Client) (string, error) {
	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return "", errors.Wrap(err, "fetching authenticated user failed")
	}
	return *user.Login, nil
}
