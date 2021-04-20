// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package publish

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v32/github"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/storage"
)

const (
	snapshotStage   = "snapshot"
	stagingStage    = "staging"
	productionStage = "production"
)

// Package function publishes the current package to the package-storage.
func Package(githubUser string, githubClient *github.Client, fork, skipPullRequest bool) error {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRoot)
	}

	buildDir, found, err := builder.FindBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "locating build directory failed")
	}
	if !found {
		return errors.New("build directory not found. Please run 'elastic-package build' first")
	}

	builtPackageDir := filepath.Join(buildDir, m.Name, m.Version)
	fmt.Printf("Use build directory: %s\n", builtPackageDir)
	_, err = os.Stat(builtPackageDir)
	if errors.Is(err, os.ErrNotExist) {
		return errors.Wrapf(err, "build directory '%s' is missing. Please run 'elastic-package build' first", builtPackageDir)
	}
	if err != nil {
		return errors.Wrapf(err, "stat file failed: %s", builtPackageDir)
	}

	fmt.Println("Clone package-storage repository")
	r, err := storage.CloneRepositoryWithFork(githubUser, productionStage, fork)
	if err != nil {
		return errors.Wrap(err, "cloning source repository failed")
	}

	fmt.Printf("Find latest package revision of \"%s\" in package-storage\n", m.Name)
	latestRevision, stage, err := findLatestPackageRevision(r, m.Name)
	if err != nil {
		return errors.Wrap(err, "can't find latest package revision")
	}

	if latestRevision == nil {
		logger.Debugf("Package has not been published yet")
	} else if latestRevision.Version == m.Version {
		fmt.Printf("Package has already been published (stage: %s, version: %s)\n", stage, latestRevision.Version)
		return nil
	} else {
		logger.Debugf("Latest package revision: %s (stage: %s)", latestRevision.String(), stage)
		logger.Debugf("Copy sources of the latest package revision to index")
	}

	fmt.Println("Check if pull request is already open")
	alreadyOpen, err := checkIfPullRequestAlreadyOpen(githubClient, *m)
	if err != nil {
		return errors.Wrapf(err, "can't check if pull request is already open")
	}
	if alreadyOpen {
		fmt.Println("Pull request with package update is already open")
		return nil
	}

	destination, err := copyLatestRevisionIfAvailable(r, latestRevision, stage, m)
	if err != nil {
		return errors.Wrap(err, "can't copy sources of latest package revision")
	}

	commitHash, err := storage.CopyOverLocalPackage(r, buildDir, m)
	if err != nil {
		return errors.Wrap(err, "can't copy over the updated package")
	}

	fmt.Println("Push new package revision to storage")
	err = storage.PushChangesWithFork(githubUser, r, fork, destination)
	if err != nil {
		return errors.Wrapf(err, "pushing changes failed")
	}

	if skipPullRequest {
		fmt.Println("Skip opening a new pull request")
		return nil
	}

	fmt.Println("Open new pull request")
	err = openPullRequest(githubClient, githubUser, destination, *m, commitHash, fork)
	if err != nil {
		return errors.Wrapf(err, "can't open a new pull request")
	}
	return nil
}

func findLatestPackageRevision(r *git.Repository, packageName string) (*storage.PackageVersion, string, error) {
	var revisions storage.PackageVersions

	revisionStageMap := map[string]string{}
	for _, currentStage := range []string{productionStage, stagingStage, snapshotStage} {
		logger.Debugf("Find revisions of the \"%s\" package in %s", packageName, currentStage)
		err := storage.ChangeStage(r, currentStage)
		if err != nil {
			return nil, "", errors.Wrapf(err, "can't change stage to %s", currentStage)
		}

		revs, err := storage.ListPackagesByName(r, packageName)
		if err != nil {
			return nil, "", errors.Wrapf(err, "can't list packages")
		}

		for _, rev := range revs {
			logger.Debugf("Found package revision: %s", rev.String())
			revisionStageMap[rev.String()] = currentStage
		}
		revisions = append(revisions, revs...)
	}

	if len(revisions) == 0 {
		logger.Debugf("No published revisions of the \"%s\" package so far. It seems to be brand new.", packageName)
		return nil, "", nil
	}

	revisions = revisions.FilterPackages(true)
	latest := revisions[0]
	return &latest, revisionStageMap[latest.String()], nil
}

func copyLatestRevisionIfAvailable(r *git.Repository, latestRevision *storage.PackageVersion, stage string, manifest *packages.PackageManifest) (string, error) {
	nonce := time.Now().UnixNano()
	destinationBranch := fmt.Sprintf("update-%s-%d", snapshotStage, nonce)
	err := storage.CopyPackagesWithTransform(r, stage, snapshotStage, optionalPackageVersions(latestRevision), destinationBranch, createRewriteResourcePath(manifest))
	if err != nil {
		return "", errors.Wrap(err, "can't copy latest revision")
	}
	return destinationBranch, nil
}

func optionalPackageVersions(pv *storage.PackageVersion) storage.PackageVersions {
	if pv != nil {
		return storage.PackageVersions{*pv}
	}
	return storage.PackageVersions{}
}

func createRewriteResourcePath(manifest *packages.PackageManifest) func(string, []byte) (string, []byte) {
	return func(resourcePath string, content []byte) (string, []byte) {
		prefix := "packages/" + manifest.Name + "/"
		if !strings.HasPrefix(resourcePath, prefix) {
			return resourcePath, content
		}
		resourcePath = resourcePath[len(prefix):]
		i := strings.IndexByte(resourcePath, '/')
		return prefix + manifest.Version + resourcePath[i:], content
	}
}
