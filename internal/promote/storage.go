package promote

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
)

type PackageRevision struct {
	Name    string
	Version string

	semver semver.Version
}

func (pr *PackageRevision) String() string {
	return fmt.Sprintf("%s-%s", pr.Name, pr.Version)
}

type PackageRevisions []PackageRevision

func (prs PackageRevisions) Strings() []string {
	var entries []string
	for _, pr := range prs {
		entries = append(entries, pr.String())
	}
	return entries
}

func CloneRepository(stage string) (*git.Repository, error) {
	r, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:           "https://github.com/elastic/package-storage",
		RemoteName:    "elastic",
		ReferenceName: plumbing.NewBranchReferenceName(stage),
	})
	if err != nil {
		return nil, errors.Wrap(err, "cloning package-storage repository failed")
	}
	return r, nil
}

func ListPackages(r *git.Repository, newestOnly bool) (PackageRevisions, error) {
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

	sort.Slice(revisions, func(i, j int) bool {
		if revisions[i].Name != revisions[j].Name {
			return sort.StringsAreSorted([]string{revisions[i].Name, revisions[j].Name})
		}
		return revisions[i].semver.LessThan(&revisions[j].semver)
	})
	return revisions, nil
}
