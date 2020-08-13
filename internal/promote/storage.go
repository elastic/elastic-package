package promote

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/github"
)

const (
	remoteName = "elastic"

	snapshotPackage = "snapshot"
	stagingPackage  = "staging"
	repositoryURL   = "https://github.com/%s/package-storage"
)

type fileContents map[string][]byte

// PackageVersion represents a package version stored in the package-storage.
type PackageVersion struct {
	Name    string
	Version string

	semver semver.Version
}

func (pr *PackageVersion) path() string {
	return filepath.Join("packages", pr.Name, pr.Version)
}

// String method returns a string representation of the PackageVersion.
func (pr *PackageVersion) String() string {
	return fmt.Sprintf("%s-%s", pr.Name, pr.Version)
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
		if v, ok := m[p.Name]; ok {
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

// CloneRepository method clones the repository and changes branch to stage.
func CloneRepository(user, stage string) (*git.Repository, error) {
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:           fmt.Sprintf(repositoryURL, "elastic"),
		RemoteName:    remoteName,
		ReferenceName: plumbing.NewBranchReferenceName(stage),
	})
	if err != nil {
		return nil, errors.Wrap(err, "cloning package-storage repository failed")
	}

	err = r.Fetch(&git.FetchOptions{
		RemoteName: remoteName,
		RefSpecs: []config.RefSpec{
			"HEAD:refs/heads/HEAD",
			"refs/heads/snapshot:refs/heads/snapshot",
			"refs/heads/staging:refs/heads/staging",
			"refs/heads/production:refs/heads/production",
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "fetch remote branches failed")
	}

	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: user,
		URLs: []string{
			fmt.Sprintf(repositoryURL, user),
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "creating remote failed")
	}
	return r, nil
}

// ListPackages method lists available packages in the package-storage.
// It skips packages: snapshot, staging.
func ListPackages(r *git.Repository) (PackageVersions, error) {
	wt, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "fetching worktree reference failed")
	}

	packageDirs, err := wt.Filesystem.ReadDir("/packages")
	if err != nil {
		return nil, errors.Wrap(err, "reading packages directory failed")
	}

	var versions PackageVersions
	for _, packageDir := range packageDirs {
		if !packageDir.IsDir() {
			continue
		}

		if packageDir.Name() == snapshotPackage || packageDir.Name() == stagingPackage {
			continue
		}

		versionDirs, err := wt.Filesystem.ReadDir(filepath.Join("/packages", packageDir.Name()))
		if err != nil {
			return nil, errors.Wrap(err, "reading packages directory failed")
		}

		for _, versionDir := range versionDirs {
			if !versionDir.IsDir() {
				continue
			}

			packageVersion, err := semver.NewVersion(versionDir.Name())
			if err != nil {
				return nil, errors.Wrapf(err, "reading package version failed (name: %s, version: %s)", packageDir.Name(), versionDir.Name())
			}

			versions = append(versions, PackageVersion{
				Name:    packageDir.Name(),
				Version: versionDir.Name(),
				semver:  *packageVersion,
			})
		}
	}
	return versions.sort(), nil
}

// DeterminePackagesToBeRemoved method lists packages supposed to be removed from the stage.
func DeterminePackagesToBeRemoved(allPackages PackageVersions, promotedPackages PackageVersions, newestOnly bool) PackageVersions {
	var removed PackageVersions

	for _, p := range allPackages {
		var toBeRemoved bool

		for _, r := range promotedPackages {
			if p.Name != r.Name {
				continue
			}

			if newestOnly {
				toBeRemoved = true
				break
			}

			if p.semver.Equal(&r.semver) {
				toBeRemoved = true
			}
		}

		if toBeRemoved {
			removed = append(removed, p)
		}
	}
	return removed
}

// CopyPackages method copies packages between branches. It creates a new branch with selected packages.
func CopyPackages(r *git.Repository, sourceStage, destinationStage string, packages PackageVersions, nonce int64) (string, error) {
	fmt.Printf("Promote packages from %s to %s...\n", sourceStage, destinationStage)

	wt, err := r.Worktree()
	if err != nil {
		return "", errors.Wrap(err, "fetching worktree reference failed")
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(sourceStage),
	})
	if err != nil {
		return "", errors.Wrapf(err, "changing branch failed (path: %s)", sourceStage)
	}

	// Load package resources from source stage
	resourcePaths, err := walkPackageVersions(wt.Filesystem, packages)
	if err != nil {
		return "", errors.Wrap(err, "walking package versions failed")
	}

	contents, err := loadPackageContents(wt.Filesystem, resourcePaths)
	if err != nil {
		return "", errors.Wrap(err, "loading package contents failed")
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(destinationStage),
	})
	if err != nil {
		return "", errors.Wrapf(err, "changing branch failed (path: %s)", destinationStage)
	}

	newDestinationStage := fmt.Sprintf("promote-from-%s-to-%s-%d", sourceStage, destinationStage, nonce)
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(newDestinationStage),
		Create: true,
	})
	if err != nil {
		return "", errors.Wrapf(err, "changing branch failed (path: %s)", newDestinationStage)
	}

	err = writePackageContents(wt.Filesystem, contents)
	if err != nil {
		return "", errors.Wrap(err, "writing package contents failed")
	}

	for resourcePath := range contents {
		_, err := wt.Add(resourcePath)
		if err != nil {
			return "", errors.Wrapf(err, "adding resource to index failed (path: %s)", resourcePath)
		}
	}

	_, err = wt.Commit(fmt.Sprintf("Promote packages from %s to %s", sourceStage, destinationStage), new(git.CommitOptions))
	if err != nil {
		return "", errors.Wrapf(err, "committing files failed (stage: %s)", newDestinationStage)
	}
	return newDestinationStage, nil
}

func walkPackageVersions(filesystem billy.Filesystem, versions PackageVersions) ([]string, error) {
	var collected []string
	for _, r := range versions {
		paths, err := walkPackageResources(filesystem, r.path())
		if err != nil {
			return nil, errors.Wrap(err, "walking package resources failed")
		}
		collected = append(collected, paths...)
	}
	return collected, nil
}

func walkPackageResources(filesystem billy.Filesystem, path string) ([]string, error) {
	fis, err := filesystem.ReadDir(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading directory failed (path: %s)", path)
	}

	var collected []string
	for _, fi := range fis {
		if fi.IsDir() {
			p := filepath.Join(path, fi.Name())
			c, err := walkPackageResources(filesystem, p)
			if err != nil {
				return nil, errors.Wrapf(err, "recursive walking failed (path: %s)", p)
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
			return nil, errors.Wrapf(err, "reading file failed (path: %s)", path)
		}

		c, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, errors.Wrapf(err, "reading file content failed (path: %s)", path)
		}

		m[path] = c
	}
	return m, nil
}

func writePackageContents(filesystem billy.Filesystem, contents fileContents) error {
	for resourcePath, content := range contents {
		dir := filepath.Dir(resourcePath)
		err := filesystem.MkdirAll(dir, 0644)
		if err != nil {
			return errors.Wrapf(err, "creating directory failed (path: %s)", dir)
		}

		err = util.WriteFile(filesystem, resourcePath, content, 0755)
		if err != nil {
			return errors.Wrapf(err, "writing file failed (path: %s)", dir)
		}
	}
	return nil
}

// RemovePackages method removes packages from "stage" branch. It creates a new branch with removed packages.
func RemovePackages(r *git.Repository, sourceStage string, packages PackageVersions, nonce int64) (string, error) {
	fmt.Printf("Remove packages from %s...\n", sourceStage)

	wt, err := r.Worktree()
	if err != nil {
		return "", errors.Wrap(err, "fetching worktree reference failed")
	}

	// Create branch for updated stage
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(sourceStage),
	})
	if err != nil {
		return "", errors.Wrapf(err, "changing branch failed (path: %s)", sourceStage)
	}

	newSourceStage := fmt.Sprintf("delete-from-%s-%d", sourceStage, nonce)
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(newSourceStage),
		Create: true,
	})
	if err != nil {
		return "", errors.Wrapf(err, "changing branch failed (path: %s)", newSourceStage)
	}

	for _, p := range packages {
		_, err := wt.Remove(p.path())
		if err != nil {
			return "", errors.Wrapf(err, "removing package from index failed (path: %s)", p.path())
		}
	}

	_, err = wt.Commit(fmt.Sprintf("Delete packages from %s", sourceStage), new(git.CommitOptions))
	if err != nil {
		return "", errors.Wrapf(err, "committing files failed (stage: %s)", sourceStage)
	}
	return newSourceStage, nil
}

// PushChanges method pushes branches to the remote repository.
func PushChanges(user string, r *git.Repository, newSourceStage, newDestinationStage string) error {
	authToken, err := github.AuthToken()
	if err != nil {
		return errors.Wrap(err, "reading auth token failed")
	}

	err = r.Push(&git.PushOptions{
		RemoteName: user,
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", newSourceStage, newSourceStage)),
			config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", newDestinationStage, newDestinationStage)),
		},
		Auth: &http.BasicAuth{
			Username: user,
			Password: authToken,
		},
	})
	if err != nil {
		return errors.Wrap(err, "pushing branch failed")
	}
	return nil
}
