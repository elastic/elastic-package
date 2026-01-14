// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestOutputOptions_NewOutputOptions(t *testing.T) {
	tests := []struct {
		name     string
		infoType string
		format   string
		wantErr  bool
	}{
		{"valid defaults", "package_name", "", false},
		{"valid json", "dir_name", "json", false},
		{"valid yaml", "absolute_path", "yaml", false},
		{"invalid info type", "invalid", "", true},
		{"invalid format", "package_name", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOutputOptions(tt.infoType, tt.format)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOutputOptions_ApplyTo(t *testing.T) {
	pkgs := []packages.PackageDirNameAndManifest{
		{
			DirName: "package1",
			Path:    "/path/to/package1",
			Manifest: &packages.PackageManifest{
				Name: "package_one",
			},
		},
		{
			DirName: "package2",
			Path:    "/path/to/package2",
			Manifest: &packages.PackageManifest{
				Name: "package_two",
			},
		},
	}

	t.Run("package_name output", func(t *testing.T) {
		opts, _ := NewOutputOptions("package_name", "")
		out, err := opts.ApplyTo(pkgs)
		require.NoError(t, err)
		assert.Contains(t, out, "package_one")
		assert.Contains(t, out, "package_two")
	})

	t.Run("dir_name output", func(t *testing.T) {
		opts, _ := NewOutputOptions("dir_name", "")
		out, err := opts.ApplyTo(pkgs)
		require.NoError(t, err)
		assert.Contains(t, out, "package1")
		assert.Contains(t, out, "package2")
	})

	t.Run("json format", func(t *testing.T) {
		opts, _ := NewOutputOptions("package_name", "json")
		out, err := opts.ApplyTo(pkgs)
		require.NoError(t, err)
		assert.Contains(t, out, `["package_one","package_two"]`)
	})

	t.Run("yaml format", func(t *testing.T) {
		opts, _ := NewOutputOptions("package_name", "yaml")
		out, err := opts.ApplyTo(pkgs)
		require.NoError(t, err)
		assert.Contains(t, out, "- package_one\n- package_two")
	})

	t.Run("absolute_path format", func(t *testing.T) {
		opts, _ := NewOutputOptions("absolute_path", "")
		out, err := opts.ApplyTo(pkgs)
		require.NoError(t, err)
		assert.Contains(t, out, "/path/to/package1")
		assert.Contains(t, out, "/path/to/package2")
	})
}
