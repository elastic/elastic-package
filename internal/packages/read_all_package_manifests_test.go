// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Layout of testdata/find_packages (depth of each package directory relative to the root):
//
//	packages/excluded/excluded_pkg       depth 3
//	packages/flat_pkg                    depth 2 (contains a data stream manifest)
//	packages/group/nested_pkg            depth 3
//	packages/group/sub/too_deep_pkg      depth 4
//	packages/invalid_manifest            depth 2 (unparsable manifest.yml)
func TestReadAllPackageManifestsFromRepo(t *testing.T) {
	searchRoot, err := filepath.Abs("testdata/find_packages")
	require.NoError(t, err)

	cases := []struct {
		name        string
		searchRoot  string
		depth       int
		excludeDirs string
		expected    []string
	}{
		{
			name:       "depth 1 finds no packages",
			searchRoot: searchRoot,
			depth:      1,
			expected:   []string{},
		},
		{
			name:       "depth 2 finds only packages directly under packages",
			searchRoot: searchRoot,
			depth:      2,
			expected:   []string{"packages/flat_pkg"},
		},
		{
			name:       "depth 3 finds packages nested under a group directory",
			searchRoot: searchRoot,
			depth:      3,
			expected: []string{
				"packages/excluded/excluded_pkg",
				"packages/flat_pkg",
				"packages/group/nested_pkg",
			},
		},
		{
			name:       "depth 4 finds packages nested under two directories",
			searchRoot: searchRoot,
			depth:      4,
			expected: []string{
				"packages/excluded/excluded_pkg",
				"packages/flat_pkg",
				"packages/group/nested_pkg",
				"packages/group/sub/too_deep_pkg",
			},
		},
		{
			name:        "excluded directories are not visited",
			searchRoot:  searchRoot,
			depth:       4,
			excludeDirs: "excluded",
			expected: []string{
				"packages/flat_pkg",
				"packages/group/nested_pkg",
				"packages/group/sub/too_deep_pkg",
			},
		},
		{
			name:        "multiple excluded directories with spaces",
			searchRoot:  searchRoot,
			depth:       4,
			excludeDirs: "excluded, group",
			expected:    []string{"packages/flat_pkg"},
		},
		{
			name:       "search root is a package",
			searchRoot: filepath.Join(searchRoot, "packages", "flat_pkg"),
			depth:      0,
			expected:   []string{"."},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pkgs, err := ReadAllPackageManifestsFromRepo(c.searchRoot, c.depth, c.excludeDirs)
			require.NoError(t, err)

			found := make([]string, 0, len(pkgs))
			for _, pkg := range pkgs {
				rel, err := filepath.Rel(c.searchRoot, pkg.Path)
				require.NoError(t, err)
				found = append(found, filepath.ToSlash(rel))

				// DirName is the package directory name and the manifest is the package one,
				// never a data stream manifest.
				assert.Equal(t, filepath.Base(pkg.Path), pkg.DirName)
				require.NotNil(t, pkg.Manifest)
				assert.Equal(t, pkg.DirName, pkg.Manifest.Name)
				assert.Contains(t, AllowedPackageTypes, pkg.Manifest.Type)
			}
			assert.Equal(t, c.expected, found)
		})
	}
}
