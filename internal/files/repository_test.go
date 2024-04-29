// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryGitDirectory(t *testing.T) {
	cases := []struct {
		name          string
		createGit     bool
		repoDir       string
		expectedError string
		valid         bool
	}{
		{
			name:          "git folder present",
			createGit:     true,
			repoDir:       t.TempDir(),
			expectedError: "",
			valid:         true,
		},
		{
			name:          "no git folder",
			createGit:     false,
			repoDir:       t.TempDir(),
			expectedError: "file does not exist",
			valid:         false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gitWorktreeFile := filepath.Join(c.repoDir, ".git")
			otherDir := filepath.Join(c.repoDir, "other")

			if c.createGit {
				err := os.MkdirAll(gitWorktreeFile, 0o755)
				require.NoError(t, err)
			}
			err := os.MkdirAll(otherDir, 0o755)
			require.NoError(t, err)

			dir, err := findRepositoryRootDirectory(otherDir)
			if c.valid {
				require.NoError(t, err)
				assert.Equal(t, c.repoDir, dir)
			} else {
				assert.ErrorContains(t, err, c.expectedError)
			}
		})
	}
}

func TestRepositoryGitWorktree(t *testing.T) {
	cases := []struct {
		name          string
		createGit     bool
		contents      string
		repoDir       string
		expectedError string
		valid         bool
	}{
		{
			name:          "valid git worktree",
			createGit:     true,
			contents:      "gitdir: /path/to/repo/main",
			repoDir:       t.TempDir(),
			expectedError: "",
			valid:         true,
		},
		{
			name:          "git worktree file with more fields",
			createGit:     true,
			contents:      "gitdir: /path/to/repo/main\nfoo: bar",
			repoDir:       t.TempDir(),
			expectedError: "",
			valid:         true,
		},
		{
			name:          "no gitdir field",
			createGit:     true,
			contents:      "",
			repoDir:       t.TempDir(),
			expectedError: "file does not exist",
			valid:         false,
		},
		{
			name:          "empty gitdir field",
			createGit:     true,
			contents:      "gitdir: ''",
			repoDir:       t.TempDir(),
			expectedError: "file does not exist",
			valid:         false,
		},
		{
			name:          "no git file nor folder",
			createGit:     false,
			contents:      "",
			repoDir:       t.TempDir(),
			expectedError: "file does not exist",
			valid:         false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gitWorktreeFile := filepath.Join(c.repoDir, ".git")
			otherDir := filepath.Join(c.repoDir, "other")

			if c.createGit {
				err := os.WriteFile(gitWorktreeFile, []byte(c.contents), 0o644)
				require.NoError(t, err)
			}
			err := os.MkdirAll(otherDir, 0o755)
			require.NoError(t, err)

			dir, err := findRepositoryRootDirectory(otherDir)
			if c.valid {
				require.NoError(t, err)
				assert.Equal(t, c.repoDir, dir)
			} else {
				assert.ErrorContains(t, err, c.expectedError)
			}
		})
	}
}
