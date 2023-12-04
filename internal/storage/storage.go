// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-billy/v5"
)

const packagesDir = "packages"

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
		return nil, fmt.Errorf("reading package version failed (name: %s, version: %s): %w", name, version, err)
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
		name, version, valid := strings.Cut(pv, "-")
		if !valid || name == "" || version == "" {
			return nil, fmt.Errorf("invalid package revision format (expected: <package_name>-<version>): %s", pv)
		}

		revision, err := NewPackageVersion(name, version)
		if err != nil {
			return nil, fmt.Errorf("can't create package version (name: %s, version: %s): %w", name, version, err)
		}
		parsed = append(parsed, *revision)
	}
	return parsed, nil
}

func walkPackageResources(filesystem billy.Filesystem, path string) ([]string, error) {
	fis, err := filesystem.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory failed (path: %s): %w", path, err)
	}

	var collected []string
	for _, fi := range fis {
		if fi.IsDir() {
			p := filepath.Join(path, fi.Name())
			c, err := walkPackageResources(filesystem, p)
			if err != nil {
				return nil, fmt.Errorf("recursive walking failed (path: %s): %w", p, err)
			}
			collected = append(collected, c...)
			continue
		}
		collected = append(collected, filepath.Join(path, fi.Name()))
	}
	return collected, nil
}
