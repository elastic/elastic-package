package github

import (
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	envAuth       = "GITHUB_TOKEN"
	authTokenFile = ".elastic/github.token"
)

// EnsureAuthConfigured method ensures that GitHub auth token is available.
func EnsureAuthConfigured() error {
	_, err := AuthToken()
	if err != nil {
		return errors.Wrapf(err, "GitHub authorization token is missing. Please use either environment variable %s or ~/%s",
			envAuth, authTokenFile)
	}
	return nil
}

// AuthToken method finds and returns the GitHub authorization token.
func AuthToken() (string, error) {
	githubTokenVar := os.Getenv(envAuth)
	if githubTokenVar != "" {
		fmt.Println("Using GitHub token from environment variable.")
		return githubTokenVar, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "reading user home directory failed")
	}

	githubTokenPath := filepath.Join(homeDir, ".elastic/github.token")
	token, err := ioutil.ReadFile(githubTokenPath)
	if err != nil {
		return "", errors.Wrapf(err, "reading Github token file failed (path: %s)", githubTokenPath)
	}
	return strings.TrimSpace(string(token)), nil
}
