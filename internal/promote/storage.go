package promote

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
)

const (
	snapshotPackage = "snapshot"
	stagingPackage  = "staging"
	repositoryURL   = "https://github.com/%s/package-storage"
)

// PackageRevision represents a package revision stored in the package-storage.
type PackageRevision struct {
	Name    string
	Version string

	semver semver.Version
}

// String method returns a string representation of the PackageRevision.
func (pr *PackageRevision) String() string {
	return fmt.Sprintf("%s-%s", pr.Name, pr.Version)
}

// PackageRevisions is an array of PackageRevision.
type PackageRevisions []PackageRevision

// Strings method returns an array of string representations.
func (prs PackageRevisions) Strings() []string {
	var entries []string
	for _, pr := range prs {
		entries = append(entries, pr.String())
	}
	return entries
}

// CloneRepository method clones the repository and changes branch to stage.
func CloneRepository(stage string) (*git.Repository, error) {
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:           fmt.Sprintf(repositoryURL, "elastic"),
		RemoteName:    "elastic",
		ReferenceName: plumbing.NewBranchReferenceName(stage),
	})
	if err != nil {
		return nil, errors.Wrap(err, "cloning package-storage repository failed")
	}

	c, err := r.Config()
	if err != nil {
		return nil, errors.Wrap(err, "reading config failed")
	}

	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: c.User.Name,
		URLs: []string{
			fmt.Sprintf(repositoryURL, c.User.Name),
		},
	})
	return r, nil
}

// ListPackages method lists available packages in the package-storage.
// It skips technical packages (snapshot, staging).
func ListPackages(r *git.Repository) (PackageRevisions, error) {
	wt, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "reading worktree failed")
	}

	packageDirs, err := wt.Filesystem.ReadDir("/packages")
	if err != nil {
		return nil, errors.Wrap(err, "reading packages directory failed")
	}

	var revisions []PackageRevision
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

			revisions = append(revisions, PackageRevision{
				Name:    packageDir.Name(),
				Version: versionDir.Name(),
				semver:  *packageVersion,
			})
		}
	}
	return sortPackageRevisions(revisions), nil
}

// FilterPackages method filters package revisions based on the "newest revision only" policy.
func FilterPackages(allPackages PackageRevisions, newestOnly bool) PackageRevisions {
	if !newestOnly {
		return allPackages
	}

	m := map[string]PackageRevision{}

	for _, p := range allPackages {
		if v, ok := m[p.Name]; ok {
			m[p.Name] = p
		} else if v.semver.LessThan(&p.semver) {
			m[p.Name] = p
		}
	}

	var revisions PackageRevisions
	for _, v := range m {
		revisions = append(revisions, v)
	}
	return sortPackageRevisions(revisions)
}

// DeterminePackagesToBeRemoved method lists packages supposed to be removed from the stage.
func DeterminePackagesToBeRemoved(allPackages PackageRevisions, promotedPackages PackageRevisions, newestOnly bool) PackageRevisions {
	var removed PackageRevisions

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
func CopyPackages(r *git.Repository, sourceStage, destinationStage string, packages PackageRevisions) (string, error) {
	return "", errors.New("CopyPackages: not implemented yet") // TODO
}

// RemovePackages method removes packages from "stage" branch. It creates a new branch with removed packages.
func RemovePackages(r *git.Repository, stage string, packages PackageRevisions) (string, error) {
	return "", errors.New("RemovePackages: not implemented yet") // TODO
}

// PushChanges method pushes branch with updated packages (updated stage) to the remote repository.
func PushChanges(r *git.Repository, stage string) error {
	return errors.New("PushChanges: not implemented yet") // TODO
}

func sortPackageRevisions(revisions PackageRevisions) PackageRevisions {
	sort.Slice(revisions, func(i, j int) bool {
		if revisions[i].Name != revisions[j].Name {
			return sort.StringsAreSorted([]string{revisions[i].Name, revisions[j].Name})
		}
		return revisions[i].semver.LessThan(&revisions[j].semver)
	})
	return revisions
}
