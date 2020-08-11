package github

import "github.com/pkg/errors"

// EnsureAuthConfigured method ensures that GitHub auth token is available.
func EnsureAuthConfigured() error {
	return errors.New("EnsureAuthToken: not implemented yet")
}

// AuthToken method finds and returns the GitHub authorization token.
func AuthToken() (string, error) {
	return "", errors.New("AuthToken: not implemented yet")
}
