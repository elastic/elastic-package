// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderLink(t *testing.T) {
	cases := []struct {
		title    string
		defs     linkMap
		caption  string
		key      string
		errors   bool
		expected string
	}{
		{
			"URLs",
			linkMap{
				map[string]string{
					"intro": "http://package-spec.test/intro",
					"docs":  "http://package-spec.test/docs",
				},
			},
			"",
			"intro",
			false,
			"http://package-spec.test/intro",
		},
		{
			"Key not exist",
			linkMap{
				map[string]string{
					"intro": "http://package-spec.test/intro",
					"docs":  "http://package-spec.test/docs",
				},
			},
			"",
			"notexist",
			true,
			"",
		},
		{
			"Markdown links",
			linkMap{
				map[string]string{
					"intro": "http://package-spec.test/intro",
					"docs":  "http://package-spec.test/docs",
				},
			},
			"Introduction",
			"intro",
			false,
			"[Introduction](http://package-spec.test/intro)",
		},
		{
			"Markdown links with code block",
			linkMap{
				map[string]string{
					"intro": "http://package-spec.test/intro",
					"docs":  "http://package-spec.test/docs",
				},
			},
			"`code`",
			"intro",
			false,
			"[`code`](http://package-spec.test/intro)",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			output, err := c.defs.RenderLink(c.key, linkOptions{caption: c.caption})
			if c.errors {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.expected, output)
		})
	}
}

func Test_linksDefinitionsFilePath(t *testing.T) {
	t.Run("env var set and file exists", func(t *testing.T) {
		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()

		defaultFilePath := filepath.Join(repoRoot.Name(), linksMapFileNameDefault)
		testFile := filepath.Join(repoRoot.Name(), "custom_links.yml")
		require.NoError(t, createLinksFile(testFile))
		require.NoError(t, createLinksFile(defaultFilePath)) // to ensure default file is ignored
		t.Setenv(linksMapFilePathEnvVar, testFile)

		path, err := linksDefinitionsFilePath(repoRoot)
		require.NoError(t, err)
		assert.Equal(t, testFile, path)
	})

	t.Run("env var set but file does not exist", func(t *testing.T) {
		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()

		missingFile := filepath.Join(repoRoot.Name(), "missing_links.yml")
		t.Setenv(linksMapFilePathEnvVar, missingFile)

		path, err := linksDefinitionsFilePath(repoRoot)
		require.Error(t, err)
		assert.Empty(t, path)
	})

	t.Run("env var not set, default file exists", func(t *testing.T) {
		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()
		defaultFilePath := filepath.Join(repoRoot.Name(), linksMapFileNameDefault)

		require.NoError(t, createLinksFile(defaultFilePath))

		path, err := linksDefinitionsFilePath(repoRoot)
		require.NoError(t, err)

		assert.Equal(t, defaultFilePath, path)
		assert.Empty(t, os.Getenv(linksMapFilePathEnvVar))
	})

	t.Run("env var not set, default file does not exist", func(t *testing.T) {
		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()
		defaultFilePath := filepath.Join(repoRoot.Name(), linksMapFileNameDefault)

		_, err = os.Stat(defaultFilePath)
		require.ErrorIs(t, err, os.ErrNotExist)

		path, err := linksDefinitionsFilePath(repoRoot)
		require.NoError(t, err)
		assert.Empty(t, path)
	})
}

func TestReadLinksMap(t *testing.T) {
	t.Run("empty path returns empty map", func(t *testing.T) {
		lmap, err := readLinksMap("")
		require.NoError(t, err)
		require.NotNil(t, lmap)
		assert.Empty(t, lmap.Links)
	})

	t.Run("non-existent file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		missingFile := filepath.Join(tmpDir, "missing.yml")
		lmap, err := readLinksMap(missingFile)
		require.Error(t, err)
		assert.Nil(t, lmap)
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "invalid.yml")
		require.NoError(t, os.WriteFile(filePath, []byte("not: valid: yaml: ["), 0644))
		lmap, err := readLinksMap(filePath)
		require.Error(t, err)
		assert.Nil(t, lmap)
	})

	t.Run("valid YAML returns populated map", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "links.yml")
		yamlContent := []byte(`links:
  intro: http://package-spec.test/intro
  docs: http://package-spec.test/docs
`)
		require.NoError(t, os.WriteFile(filePath, yamlContent, 0644))
		lmap, err := readLinksMap(filePath)
		require.NoError(t, err)
		require.NotNil(t, lmap)
		assert.Equal(t, "http://package-spec.test/intro", lmap.Links["intro"])
		assert.Equal(t, "http://package-spec.test/docs", lmap.Links["docs"])
	})

	t.Run("valid YAML with empty links returns empty map", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "empty.yml")
		yamlContent := []byte("links: {}\n")
		require.NoError(t, os.WriteFile(filePath, yamlContent, 0644))
		lmap, err := readLinksMap(filePath)
		require.NoError(t, err)
		require.NotNil(t, lmap)
		assert.Empty(t, lmap.Links)
	})
}

func createLinksFile(filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}
