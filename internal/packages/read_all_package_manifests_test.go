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
//	packages/flat_pkg/_dev/decoy_pkg     depth 4 (package manifest inside another package, never reported)
//	packages/group/nested_pkg            depth 3
//	packages/group/sub/too_deep_pkg      depth 4
//	packages/invalid_manifest            depth 2 (unparsable manifest.yml)
func TestReadAllPackageManifestsFromRepo(t *testing.T) {
	fixtureRoot, err := filepath.Abs("testdata/find_packages")
	require.NoError(t, err)

	// Ensure the decoy package exists, so the cases below fail if it is ever reported.
	require.FileExists(t, filepath.Join(fixtureRoot, "packages", "flat_pkg", "_dev", "decoy_pkg", PackageManifestFile))

	cases := []struct {
		name string
		// searchRoot is relative to the fixture root; empty means the fixture root itself.
		searchRoot  string
		depth       int
		excludeDirs string
		expected    []string
	}{
		{
			name:     "depth 1 finds no packages",
			depth:    1,
			expected: []string{},
		},
		{
			name:     "depth 2 finds only packages directly under packages",
			depth:    2,
			expected: []string{"packages/flat_pkg"},
		},
		{
			name:  "depth 3 finds packages nested under a group directory",
			depth: 3,
			expected: []string{
				"packages/excluded/excluded_pkg",
				"packages/flat_pkg",
				"packages/group/nested_pkg",
			},
		},
		{
			name:  "depth 4 finds packages nested under two directories",
			depth: 4,
			expected: []string{
				"packages/excluded/excluded_pkg",
				"packages/flat_pkg",
				"packages/group/nested_pkg",
				"packages/group/sub/too_deep_pkg",
			},
		},
		{
			name:  "packages inside another package are never reported",
			depth: 10,
			expected: []string{
				"packages/excluded/excluded_pkg",
				"packages/flat_pkg",
				"packages/group/nested_pkg",
				"packages/group/sub/too_deep_pkg",
			},
		},
		{
			name:        "excluded directories are not visited",
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
			depth:       4,
			excludeDirs: "excluded, group",
			expected:    []string{"packages/flat_pkg"},
		},
		{
			name:       "search root is a package",
			searchRoot: filepath.Join("packages", "flat_pkg"),
			depth:      0,
			expected:   []string{"."},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := filepath.Join(fixtureRoot, c.searchRoot)
			pkgs, err := ReadAllPackageManifestsFromRepo(root, c.depth, c.excludeDirs)
			require.NoError(t, err)

			found := make([]string, 0, len(pkgs))
			for _, pkg := range pkgs {
				rel, err := filepath.Rel(root, pkg.Path)
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
