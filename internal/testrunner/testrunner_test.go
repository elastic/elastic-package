// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The package roots are built with filepath.Join to be clean paths with the separators of the system,
// as the package roots found by elastic-package are, since the folders found are compared to them.
var (
	// apache_basic_license is an integration package whose folder name differs from its manifest name (apache).
	integrationPackageRoot = filepath.Join("..", "..", "test", "packages", "parallel", "apache_basic_license")
	integrationPackageName = "apache"

	// 01_ci_input_pkg is an input package whose folder name differs from its manifest name (ci_input_pkg).
	inputPackageRoot = filepath.Join("..", "..", "test", "packages", "composable", "01_ci_input_pkg")
	inputPackageName = "ci_input_pkg"
)

func TestFindTestFolders_PackageName(t *testing.T) {
	t.Run("integration package", func(t *testing.T) {
		folders, err := FindTestFolders(integrationPackageRoot, nil, "pipeline")
		require.NoError(t, err)
		require.NotEmpty(t, folders)

		for _, folder := range folders {
			assert.Equal(t, integrationPackageName, folder.Package)
			assert.NotEmpty(t, folder.DataStream)
		}
	})

	t.Run("integration package with data streams", func(t *testing.T) {
		folders, err := FindTestFolders(integrationPackageRoot, []string{"access"}, "pipeline")
		require.NoError(t, err)
		require.Len(t, folders, 1)

		assert.Equal(t, integrationPackageName, folders[0].Package)
		assert.Equal(t, "access", folders[0].DataStream)
	})

	t.Run("input package", func(t *testing.T) {
		// This package has no test type folders, an empty test type matches all the entries in _dev/test.
		folders, err := FindTestFolders(inputPackageRoot, nil, "")
		require.NoError(t, err)
		require.Len(t, folders, 1)

		assert.Equal(t, inputPackageName, folders[0].Package)
		assert.Empty(t, folders[0].DataStream)
	})

	t.Run("missing manifest", func(t *testing.T) {
		packageRoot := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(packageRoot, "_dev", "test", "system"), 0o755))

		_, err := FindTestFolders(packageRoot, nil, "system")
		require.Error(t, err)
		assert.ErrorContains(t, err, "reading package manifest failed")
	})
}

func TestAssumeTestFolders_PackageName(t *testing.T) {
	t.Run("all data streams", func(t *testing.T) {
		folders, err := AssumeTestFolders(integrationPackageRoot, nil, "static")
		require.NoError(t, err)
		require.NotEmpty(t, folders)

		for _, folder := range folders {
			assert.Equal(t, integrationPackageName, folder.Package)
			assert.NotEmpty(t, folder.DataStream)
		}
	})

	t.Run("selected data streams", func(t *testing.T) {
		folders, err := AssumeTestFolders(integrationPackageRoot, []string{"access", "error"}, "static")
		require.NoError(t, err)
		require.Len(t, folders, 2)

		for _, folder := range folders {
			assert.Equal(t, integrationPackageName, folder.Package)
		}
	})

	t.Run("missing manifest", func(t *testing.T) {
		packageRoot := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(packageRoot, "data_stream", "logs"), 0o755))

		_, err := AssumeTestFolders(packageRoot, nil, "static")
		require.Error(t, err)
		assert.ErrorContains(t, err, "reading package manifest failed")
	})
}
