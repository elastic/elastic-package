// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"archive/zip"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicensesOnBuiltPackage(t *testing.T) {
	packagePaths := []string{
		"../../test/packages/parallel/apache",
		"../../test/packages/other/sql_input",
	}

	for _, packageRoot := range packagePaths {
		t.Run(path.Base(packageRoot), func(t *testing.T) {
			options := BuildOptions{
				PackageRoot: packageRoot,
			}
			buildDir := t.TempDir()
			buildPath, err := buildPackageWithBuildDir(options, buildDir)
			require.NoError(t, err)
			assertRelativePath(t, buildDir, buildPath)
			assert.FileExists(t, filepath.Join(buildPath, "LICENSE.txt"))
		})

		t.Run(path.Base(packageRoot)+".zip", func(t *testing.T) {
			options := BuildOptions{
				PackageRoot: packageRoot,
				CreateZip:   true,
			}
			buildDir := t.TempDir()
			buildPath, err := buildPackageWithBuildDir(options, buildDir)
			require.NoError(t, err)
			assertRelativePath(t, buildDir, buildPath)

			r, err := zip.OpenReader(buildPath)
			require.NoError(t, err)
			defer r.Close()

			rootDir := strings.TrimSuffix(filepath.Base(buildPath), ".zip")
			f, err := r.Open(path.Join(rootDir, "LICENSE.txt"))
			require.NoError(t, err)
			defer f.Close()
		})
	}
}

func assertRelativePath(t *testing.T, basepath, path string) {
	t.Helper()
	isRelativePath := strings.HasPrefix(filepath.Clean(path), filepath.Clean(basepath))
	assert.Truef(t, isRelativePath, "%q should be a path under %q", path, basepath)
}
