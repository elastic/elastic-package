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

func TestRepositoryDirectory(t *testing.T) {
	tempDir := t.TempDir()

	gitDir := filepath.Join(tempDir, ".git")
	otherDir := filepath.Join(tempDir, "other")

	err := os.MkdirAll(gitDir, 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(otherDir, 0o755)
	require.NoError(t, err)

	err = os.Chdir(otherDir)
	require.NoError(t, err)

	dir, err := FindRepositoryRootDirectory()
	require.NoError(t, err)
	assert.Equal(t, tempDir, dir)

	// test a non repository folder
	nonGitDir := t.TempDir()

	err = os.Chdir(nonGitDir)
	require.NoError(t, err)

	_, err = FindRepositoryRootDirectory()
	assert.ErrorIs(t, err, os.ErrNotExist)
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
			name:          "invalid git worktree file",
			createGit:     true,
			contents:      "gitdir: /path/to/repo/main\nfoo: bar",
			repoDir:       t.TempDir(),
			expectedError: "yaml: unmarshal errors:\n  line 2: field foo not found in type files.gitWorktree",
			valid:         false,
		},
		{
			name:          "invalid git worktree file",
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

			err = os.Chdir(otherDir)
			require.NoError(t, err)

			dir, err := FindRepositoryRootDirectory()
			if c.valid {
				require.NoError(t, err)
				assert.Equal(t, c.repoDir, dir)
			} else {
				assert.ErrorContains(t, err, "yaml: unmarshal errors:\n  line 2: field foo not found in type files.gitWorktree")
			}
		})
	}
}
