// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func isGitWorktree(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	type gitWorktree struct {
		GitDir string `yaml:"gitdir"`
	}
	var worktree gitWorktree
	dec := yaml.NewDecoder(bytes.NewBuffer(content))
	dec.KnownFields(true)

	if err := dec.Decode(&worktree); err != nil {
		return fmt.Errorf("failed to decode %s: %w", path, err)
	}

	return nil
}

func FindRepositoryRootDirectory() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locating working directory failed: %w", err)
	}

	// VolumeName() will return something like "C:" in Windows, and "" in other OSs
	// rootDir will be something like "C:\" in Windows, and "/" everywhere else.
	rootDir := filepath.VolumeName(workDir) + string(filepath.Separator)

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, ".git")
		fileInfo, err := os.Stat(path)
		if err == nil && !fileInfo.IsDir() {
			errWorktree := isGitWorktree(path)
			if errWorktree != nil {
				return "", errWorktree
			}

			return dir, nil
		}
		if err == nil && fileInfo.IsDir() {
			return dir, nil
		}

		if dir == rootDir {
			break
		}
		dir = filepath.Dir(dir)
	}

	return "", os.ErrNotExist
}
