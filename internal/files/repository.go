// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func FindRepositoryRoot() (*os.Root, error) {
	rootPath, err := FindRepositoryRootDirectory()
	if err != nil {
		return nil, fmt.Errorf("root not found: %w", err)
	}

	// scope any possible operation to the repository folder
	dirRoot, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("could not open root: %w", err)
	}

	return dirRoot, nil
}

func FindRepositoryRootDirectory() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locating working directory failed: %w", err)
	}
	return findRepositoryRootDirectory(workDir)
}

func findRepositoryRootDirectory(workDir string) (string, error) {
	// VolumeName() will return something like "C:" in Windows, and "" in other OSs
	// rootDir will be something like "C:\" in Windows, and "/" everywhere else.
	rootDir := filepath.VolumeName(workDir) + string(filepath.Separator)

	dir := workDir
	for dir != "." {
		gitRepo, err := isGitRootDir(dir)
		if err != nil {
			return "", err
		}
		if gitRepo {
			return dir, nil
		}
		if dir == rootDir {
			break
		}
		dir = filepath.Dir(dir)
	}

	return "", os.ErrNotExist
}

func isGitRootDir(dir string) (bool, error) {
	path := filepath.Join(dir, ".git")
	fileInfo, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if fileInfo.IsDir() {
		return true, nil
	}

	worktree, err := isGitWorktree(path)
	if err != nil {
		return false, err
	}

	return worktree, nil
}

func isGitWorktree(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", path, err)
	}
	type gitWorktree struct {
		GitDir string `yaml:"gitdir"`
	}
	var worktree gitWorktree
	if err := yaml.Unmarshal(content, &worktree); err != nil {
		return false, fmt.Errorf("failed to unmarshall %s: %w", path, err)
	}

	return worktree.GitDir != "", nil
}
