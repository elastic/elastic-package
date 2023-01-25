// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/elastic/elastic-package/internal/github"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

const (
	upstream = "elastic"

	snapshotPackage = "snapshot"
	stagingPackage  = "staging"
	repositoryURL   = "https://github.com/%s/package-storage"

	packagesDir = "packages"
)

type fileContents map[string][]byte

type contentTransformer func(string, []byte) (string, []byte)

// PackageVersion represents a package version stored in the package-storage.
type PackageVersion struct {
	Name    string
	Version string

	root   string
	semver semver.Version
}

// NewPackageVersion function creates an instance of PackageVersion.
func NewPackageVersion(name, version string) (*PackageVersion, error) {
	return NewPackageVersionWithRoot(name, version, packagesDir)
}

// NewPackageVersionWithRoot function creates an instance of PackageVersion and defines a custom root.
func NewPackageVersionWithRoot(name, version, root string) (*PackageVersion, error) {
	packageVersion, err := semver.NewVersion(version)
	if err != nil {
		return nil, fmt.Errorf("reading package version failed (name: %s, version: %s): %s", name, version, err)
	}
	return &PackageVersion{
		Name:    name,
		Version: version,
		root:    root,
		semver:  *packageVersion,
	}, nil
}

func (pv *PackageVersion) path() string {
	if pv.root != "" {
		return filepath.Join(pv.root, pv.Name, pv.Version)
	}
	return filepath.Join(pv.Name, pv.Version)
}

// Equal method can be used to compare two PackageVersions.
func (pv *PackageVersion) Equal(other PackageVersion) bool {
	return pv.semver.Equal(&other.semver) && pv.Name == other.Name
}

// String method returns a string representation of the PackageVersion.
func (pv *PackageVersion) String() string {
	return fmt.Sprintf("%s-%s", pv.Name, pv.Version)
}

// PackageVersions is an array of PackageVersion.
type PackageVersions []PackageVersion

// FilterPackages method filters package versions based on the "newest version only" policy.
func (prs PackageVersions) FilterPackages(newestOnly bool) PackageVersions {
	if !newestOnly {
		return prs
	}

	m := map[string]PackageVersion{}

	for _, p := range prs {
		if v, ok := m[p.Name]; !ok {
			m[p.Name] = p
		} else if v.semver.LessThan(&p.semver) {
			m[p.Name] = p
		}
	}

	var versions PackageVersions
	for _, v := range m {
		versions = append(versions, v)
	}
	return versions.sort()
}

func (prs PackageVersions) sort() PackageVersions {
	sort.Slice(prs, func(i, j int) bool {
		if prs[i].Name != prs[j].Name {
			return sort.StringsAreSorted([]string{prs[i].Name, prs[j].Name})
		}
		return prs[i].semver.LessThan(&prs[j].semver)
	})
	return prs
}

// Strings method returns an array of string representations.
func (prs PackageVersions) Strings() []string {
	var entries []string
	for _, pr := range prs {
		entries = append(entries, pr.String())
	}
	return entries
}

// ParsePackageVersions function parses string representation of revisions into structure.
func ParsePackageVersions(packageVersions []string) (PackageVersions, error) {
	var parsed PackageVersions
	for _, pv := range packageVersions {
		s := strings.Split(pv, "-")
		if len(s) != 2 {
			return nil, fmt.Errorf("invalid package revision format (expected: <package_name>-<version>): %s", pv)
		}

		revision, err := NewPackageVersion(s[0], s[1])
		if err != nil {
			return nil, fmt.Errorf("can't create package version (%s): %s", s, err)
		}
		parsed = append(parsed, *revision)
	}
	return parsed, nil
}

// CloneRepository function clones the repository and changes branch to stage.
// It assumes that user has already forked the storage repository.
func CloneRepository(user, stage string) (*git.Repository, error) {
	return CloneRepositoryWithFork(user, stage, true)
}

// CloneRepositoryWithFork function clones the repository, changes branch to stage.
// It respects the fork mode accordingly.
func CloneRepositoryWithFork(user, stage string, fork bool) (*git.Repository, error) {
	// Initialize repository
	r, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		return nil, fmt.Errorf("initializing repository: %s", err)
	}

	// Add remotes
	userRepositoryURL := fmt.Sprintf(repositoryURL, user)
	var userRemote *git.Remote
	if !fork {
		logger.Debugf("No-fork mode selected. The user's remote upstream won't be created.")
	} else {
		userRemote, err = r.CreateRemote(&config.RemoteConfig{
			Name: user,
			URLs: []string{
				userRepositoryURL,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("creating user remote failed: %s", err)
		}
	}

	upstreamRemote, err := r.CreateRemote(&config.RemoteConfig{
		Name: upstream,
		URLs: []string{
			fmt.Sprintf(repositoryURL, upstream),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating remote failed: %s", err)
	}

	// Check if user remote exists
	authToken, err := github.AuthToken()
	if err != nil {
		return nil, fmt.Errorf("reading auth token failed: %s", err)
	}

	if !fork {
		logger.Debugf("No-fork mode selected. The user's remote upstream won't be listed.")
	} else {
		_, err = userRemote.List(&git.ListOptions{
			Auth: &http.BasicAuth{
				Username: user,
				Password: authToken,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("checking user remote (%s, url: %s): %s", user, userRepositoryURL, err)
		}
	}

	// Fetch and checkout
	err = upstreamRemote.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{
			"refs/heads/snapshot:refs/heads/snapshot",
			"refs/heads/staging:refs/heads/staging",
			"refs/heads/production:refs/heads/production",
		},
		Depth: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch remote branches failed: %s", err)
	}
	wt, err := r.Worktree()
	if err != nil {
		return nil, fmt.Errorf("working copy initialization failed: %s", err)
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(stage),
	})
	if err != nil {
		return nil, fmt.Errorf("checkout failed: %s", err)
	}

	return r, nil
}

// ChangeStage function selects the stage in the package storage.
func ChangeStage(r *git.Repository, stage string) error {
	wt, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("fetching worktree reference failed: %s", err)
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(stage),
	})
	if err != nil {
		return fmt.Errorf("changing branch failed (stage: %s): %s", stage, err)
	}

	err = wt.Clean(&git.CleanOptions{
		Dir: true,
	})
	if err != nil {
		return fmt.Errorf("can't hard reset worktree (stage: %s): %s", stage, err)
	}
	return nil
}

// ListPackages function lists available packages in the package-storage.
func ListPackages(r *git.Repository) (PackageVersions, error) {
	return ListPackagesByName(r, "")
}

// ListPackagesByName function lists available packages in the package-storage.
// It filters packages by name and skips packages: snapshot, staging.
func ListPackagesByName(r *git.Repository, packageName string) (PackageVersions, error) {
	wt, err := r.Worktree()
	if err != nil {
		return nil, fmt.Errorf("fetching worktree reference failed: %s", err)
	}

	packageDirs, err := wt.Filesystem.ReadDir("/" + packagesDir)
	if err != nil {
		return nil, fmt.Errorf("reading packages directory failed: %s", err)
	}

	var versions PackageVersions
	for _, packageDir := range packageDirs {
		if !packageDir.IsDir() {
			continue
		}

		if packageDir.Name() == snapshotPackage || packageDir.Name() == stagingPackage {
			continue
		}

		if packageName != "" && packageName != packageDir.Name() {
			continue
		}

		versionDirs, err := wt.Filesystem.ReadDir(filepath.Join(packagesDir, packageDir.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading packages directory failed: %s", err)
		}

		for _, versionDir := range versionDirs {
			if !versionDir.IsDir() {
				continue
			}

			packageVersion, err := NewPackageVersion(packageDir.Name(), versionDir.Name())
			if err != nil {
				return nil, fmt.Errorf("can't create instance of PackageVersion: %s", err)
			}
			versions = append(versions, *packageVersion)
		}
	}
	return versions.sort(), nil
}

// CopyPackages function copies packages between branches. It creates a new branch with selected packages.
func CopyPackages(r *git.Repository, sourceStage, destinationStage string, packages PackageVersions, destinationBranch string) error {
	return CopyPackagesWithTransform(r, sourceStage, destinationStage, packages, destinationBranch, nil)
}

// CopyPackagesWithTransform function copies packages between branches and modifies file content using transform function.
// It creates a new branch with selected packages.
// The function doesn't fail if the source stage doesn't exist.
func CopyPackagesWithTransform(r *git.Repository, sourceStage, destinationStage string, packages PackageVersions, destinationBranch string,
	transform contentTransformer) error {
	wt, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("fetching worktree reference failed: %s", err)
	}

	var contents fileContents
	if sourceStage != "" {
		logger.Debugf("Checkout source stage: %s", sourceStage)
		err = wt.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(sourceStage),
		})
		if err != nil {
			return fmt.Errorf("changing branch failed (path: %s): %s", sourceStage, err)
		}

		logger.Debugf("Load package resources from source stage")
		resourcePaths, err := walkPackageVersions(wt.Filesystem, packages...)
		if err != nil {
			return fmt.Errorf("walking package versions failed: %s", err)
		}

		contents, err = loadPackageContents(wt.Filesystem, resourcePaths)
		if err != nil {
			return fmt.Errorf("loading package contents failed: %s", err)
		}
	}

	logger.Debugf("Checkout destination stage: %s", destinationStage)
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(destinationStage),
	})
	if err != nil {
		return fmt.Errorf("changing branch failed (path: %s): %s", destinationStage, err)
	}

	logger.Debugf("Create new destination branch: %s", destinationBranch)
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(destinationBranch),
		Create: true,
	})
	if err != nil {
		return fmt.Errorf("changing branch failed (path: %s): %s", destinationBranch, err)
	}

	if len(contents) == 0 {
		return nil
	}

	if transform != nil {
		contents = transformPackageContents(contents, transform)
	}

	logger.Debugf("Write package resources to destination branch")
	err = writePackageContents(wt.Filesystem, contents)
	if err != nil {
		return fmt.Errorf("writing package contents failed: %s", err)
	}

	logger.Debugf("Add package resources to index")
	_, err = wt.Add(packagesDir)
	if err != nil {
		return fmt.Errorf("adding resource to index failed: %s", err)
	}

	logger.Debugf("Commit changes to destination branch")
	_, err = wt.Commit(fmt.Sprintf("Copy packages from %s to %s", sourceStage, destinationStage), new(git.CommitOptions))
	if err != nil {
		return fmt.Errorf("committing files failed (stage: %s): %s", destinationBranch, err)
	}
	return nil
}

// CopyOverLocalPackage function updates the local repository with the selected local package.
// It returns the commit hash for the HEAD.
//
// Principle of operation
// 0. Git index is clean.
// 1. All files need to be removed from the destination folder (in Git repository).
// 2. Copy package content to the destination folder.
//
// Result:
// The destination folder contains new/updated files and doesn't contain removed ones.
func CopyOverLocalPackage(r *git.Repository, builtPackageDir string, manifest *packages.PackageManifest) (string, error) {
	wt, err := r.Worktree()
	if err != nil {
		return "", fmt.Errorf("fetching worktree reference failed: %s", err)
	}

	logger.Debugf("Temporarily remove all files from index")
	publishedPackageDir := filepath.Join(packagesDir, manifest.Name, manifest.Version)
	_, err = wt.Remove(publishedPackageDir)
	if err != nil && err != index.ErrEntryNotFound {
		return "", fmt.Errorf("can't remove files within path: %s: %s", publishedPackageDir, err)
	}

	packageVersion, err := NewPackageVersionWithRoot(manifest.Name, manifest.Version, "")
	if err != nil {
		return "", fmt.Errorf("can't create instance of PackageVersion: %s", err)
	}

	logger.Debugf("Evaluate all resource paths for the package (buildDir: %s)", builtPackageDir)
	osFs := osfs.New(builtPackageDir)
	resourcePaths, err := walkPackageVersions(osFs, *packageVersion)
	if err != nil {
		return "", fmt.Errorf("walking package versions failed: %s", err)
	}

	contents, err := loadPackageContents(osFs, resourcePaths)
	if err != nil {
		return "", fmt.Errorf("loading package contents failed: %s", err)
	}

	contents = transformPackageContents(contents, func(path string, body []byte) (string, []byte) {
		return filepath.Join(packagesDir, path), body
	})

	err = writePackageContents(wt.Filesystem, contents)
	if err != nil {
		return "", fmt.Errorf("writing package contents failed: %s", err)
	}

	logger.Debugf("Add updated resources to index")
	_, err = wt.Add(packagesDir)
	if err != nil {
		return "", fmt.Errorf("adding updated resource to index failed: %s", err)
	}

	logger.Debugf("Commit changes to destination branch")
	commitHash, err := wt.Commit("Copy over local package sources", new(git.CommitOptions))
	if err != nil {
		return "", fmt.Errorf("committing files failed: %s", err)
	}
	return commitHash.String(), nil
}

func walkPackageVersions(filesystem billy.Filesystem, versions ...PackageVersion) ([]string, error) {
	var collected []string
	for _, r := range versions {
		paths, err := walkPackageResources(filesystem, r.path())
		if err != nil {
			return nil, fmt.Errorf("walking package resources failed: %s", err)
		}
		collected = append(collected, paths...)
	}
	return collected, nil
}

func walkPackageResources(filesystem billy.Filesystem, path string) ([]string, error) {
	fis, err := filesystem.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory failed (path: %s): %s", path, err)
	}

	var collected []string
	for _, fi := range fis {
		if fi.IsDir() {
			p := filepath.Join(path, fi.Name())
			c, err := walkPackageResources(filesystem, p)
			if err != nil {
				return nil, fmt.Errorf("recursive walking failed (path: %s): %s", p, err)
			}
			collected = append(collected, c...)
			continue
		}
		collected = append(collected, filepath.Join(path, fi.Name()))
	}
	return collected, nil
}

func loadPackageContents(filesystem billy.Filesystem, resourcePaths []string) (fileContents, error) {
	m := fileContents{}
	for _, path := range resourcePaths {
		f, err := filesystem.Open(path)
		if err != nil {
			return nil, fmt.Errorf("reading file failed (path: %s): %s", path, err)
		}

		c, err := io.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("reading file content failed (path: %s): %s", path, err)
		}

		m[path] = c
	}
	return m, nil
}

func transformPackageContents(contents fileContents, transform contentTransformer) fileContents {
	transformed := fileContents{}
	for r, c := range contents {
		dr, dc := transform(r, c)
		transformed[dr] = dc
	}
	return transformed
}

func writePackageContents(filesystem billy.Filesystem, contents fileContents) error {
	for resourcePath, content := range contents {
		dir := filepath.Dir(resourcePath)
		err := filesystem.MkdirAll(dir, 0644)
		if err != nil {
			return fmt.Errorf("creating directory failed (path: %s): %s", dir, err)
		}

		err = util.WriteFile(filesystem, resourcePath, content, 0755)
		if err != nil {
			return fmt.Errorf("writing file failed (path: %s): %s", dir, err)
		}
	}
	return nil
}

// RemovePackages function removes packages from "stage" branch. It creates a new branch with removed packages.
func RemovePackages(r *git.Repository, sourceStage string, packages PackageVersions, sourceBranch string) error {
	fmt.Printf("Remove packages from %s...\n", sourceStage)

	wt, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("fetching worktree reference failed: %s", err)
	}

	logger.Debugf("Checkout source stage: %s", sourceStage)
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(sourceStage),
	})
	if err != nil {
		return fmt.Errorf("changing branch failed (path: %s): %s", sourceStage, err)
	}

	logger.Debugf("Create new source branch: %s", sourceBranch)
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(sourceBranch),
		Create: true,
	})
	if err != nil {
		return fmt.Errorf("changing branch failed (path: %s): %s", sourceBranch, err)
	}

	logger.Debugf("Remove package resources from new source branch")
	for _, p := range packages {
		_, err := wt.Remove(p.path())
		if err != nil {
			return fmt.Errorf("removing package from index failed (path: %s): %s", p.path(), err)
		}
	}

	logger.Debugf("Commit changes to new source branch")
	_, err = wt.Commit(fmt.Sprintf("Delete packages from %s", sourceStage), new(git.CommitOptions))
	if err != nil {
		return fmt.Errorf("committing files failed (stage: %s): %s", sourceStage, err)
	}
	return nil
}

// PushChanges function pushes branches to the remote repository.
// It assumes that user has already forked the storage repository.
func PushChanges(user string, r *git.Repository, stages ...string) error {
	return PushChangesWithFork(user, r, true, stages...)
}

// PushChangesWithFork function pushes branches to the remote repository.
// It respects the fork mode accordingly.
func PushChangesWithFork(user string, r *git.Repository, fork bool, stages ...string) error {
	authToken, err := github.AuthToken()
	if err != nil {
		return fmt.Errorf("reading auth token failed: %s", err)
	}

	var refSpecs []config.RefSpec
	for _, stage := range stages {
		refSpecs = append(refSpecs, config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", stage, stage)))
	}

	remoteName := upstream
	if fork {
		remoteName = user
	}

	logger.Debugf("Push to remote: %s", remoteName)
	err = r.Push(&git.PushOptions{
		RemoteName: remoteName,
		RefSpecs:   refSpecs,
		Auth: &http.BasicAuth{
			Username: user,
			Password: authToken,
		},
	})
	if err != nil {
		return fmt.Errorf("pushing branch failed: %s", err)
	}
	return nil
}
