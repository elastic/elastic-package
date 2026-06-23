// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	builtPackageDir            = "testdata/built_package"
	sourcePackageDir           = "testdata/source_package"
	composableSourcePackageDir = "testdata/composable_source_package"
)

func TestValidateSourceFromPath(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		expectError bool
	}{
		// source_package has _dev/; ModeSource allows it.
		{name: "valid source package", dir: sourcePackageDir},
		// built_package has _embedded_ecs dynamic templates auto-injected at build time;
		// ModeSource rejects them via ValidateNoEmbeddedEcsInDynamicTemplates, confirming
		// the built fixture is meaningfully distinct from the source fixture.
		{name: "rejects _embedded_ecs dynamic templates", dir: builtPackageDir, expectError: true},
		// composable_source_package uses package: in stream definitions; ModeSource allows it.
		{name: "valid composable source package", dir: composableSourcePackageDir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationErr, filteredErr := ValidateSourceFromPath(tt.dir)
			if tt.expectError {
				assert.Error(t, validationErr)
				return
			}
			assert.NoError(t, validationErr)
			assert.NoError(t, filteredErr)
		})
	}
}

func TestValidateBuiltFromPath(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		expectError bool
	}{
		// built_package has no _dev/ or source-only artifacts; ModeBuild accepts it.
		{name: "valid built package", dir: builtPackageDir},
		// source_package has _dev/; ModeBuild rejects it via ValidateNoDevFolder.
		// Guards against regression if ValidateBuiltFromPath is ever switched back to ModeLegacy.
		{name: "rejects _dev/ directory", dir: sourcePackageDir, expectError: true},
		// composable_source_package has package: in stream definitions and policy template inputs;
		// ModeBuild rejects these via ValidateStreamInputMaterialized.
		{name: "rejects un-materialized package: references", dir: composableSourcePackageDir, expectError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationErr, filteredErr := ValidateBuiltFromPath(tt.dir)
			if tt.expectError {
				assert.Error(t, validationErr)
				return
			}
			assert.NoError(t, validationErr)
			assert.NoError(t, filteredErr)
		})
	}
}

func TestValidateBuiltFromZip(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		expectError bool
	}{
		// built_package has no _dev/ or source-only artifacts; ModeBuild accepts it.
		{name: "valid built package", dir: builtPackageDir},
		// source_package has _dev/; ModeBuild rejects it via ValidateNoDevFolder.
		// Guards the zip path the same way TestValidateBuiltFromPath guards the directory path.
		{name: "rejects _dev/ directory", dir: sourcePackageDir, expectError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zipPath := makeZipFromDir(t, tt.dir)
			validationErr, filteredErr := ValidateBuiltFromZip(zipPath)
			if tt.expectError {
				assert.Error(t, validationErr)
				return
			}
			assert.NoError(t, validationErr)
			assert.NoError(t, filteredErr)
		})
	}
}

// makeZipFromDir zips dir into a temp file with the package directory as the single
// top-level entry, matching the layout that fsFromPackageZip expects.
func makeZipFromDir(t *testing.T, dir string) string {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), filepath.Base(dir)+".zip")
	f, err := os.Create(tmp)
	require.NoError(t, err)
	defer f.Close()

	w := zip.NewWriter(f)
	base := filepath.Base(dir)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		fw, err := w.Create(base + "/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = fw.Write(data)
		return err
	})
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return tmp
}
