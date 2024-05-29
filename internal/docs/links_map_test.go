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

func TestLinksDefinitionsFilePath(t *testing.T) {
	currentDirectory, _ := os.Getwd()
	temporalDirecotry := t.TempDir()

	cases := []struct {
		title                string
		createFileFromEnvVar bool
		createDefaultFile    bool
		linksFilePath        string
		expectedErrors       bool
		expected             string
	}{
		{
			title:                "No env var and no default file",
			createFileFromEnvVar: false,
			createDefaultFile:    false,
			linksFilePath:        "",
			expectedErrors:       false,
			expected:             "",
		},
		{
			title:                "No env var - default file",
			createFileFromEnvVar: false,
			createDefaultFile:    true,
			linksFilePath:        "",
			expectedErrors:       false,
			expected:             filepath.Join(currentDirectory, "links_table.yml"),
		},
		{
			title:                "Env var defined",
			createFileFromEnvVar: true,
			createDefaultFile:    false,
			linksFilePath:        filepath.Join(temporalDirecotry, "links_table.yml"),
			expectedErrors:       false,
			expected:             filepath.Join(temporalDirecotry, "links_table.yml"),
		},
		{
			title:                "Env var defined but just default file exists",
			createFileFromEnvVar: false,
			createDefaultFile:    true,
			linksFilePath:        filepath.Join(temporalDirecotry, "links_table_2.yml"),
			expectedErrors:       true,
			expected:             "",
		},
	}

	createGitFolder()
	defer removeGitFolder()

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			var err error
			if c.linksFilePath != "" {
				err = os.Setenv(linksMapFilePathEnvVar, c.linksFilePath)
				require.NoError(t, err)
				defer os.Unsetenv(linksMapFilePathEnvVar)
			}

			if c.createFileFromEnvVar {
				err = createLinksFile(c.linksFilePath)
				defer removeLinksFile(c.linksFilePath)
				require.NoError(t, err)
			}

			if c.createDefaultFile {
				err = createLinksFile(linksMapFileNameDefault)
				require.NoError(t, err)
				defer removeLinksFile(linksMapFileNameDefault)
			}

			d := NewDocsRenderer()
			path, err := d.linksDefinitionsFilePath()

			if c.expectedErrors {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, c.expected, path)
			}
		})
	}
}

func createGitFolder() error {
	return os.MkdirAll(".git", os.ModePerm)
}

func removeGitFolder() error {
	return os.RemoveAll(".git")
}

func createLinksFile(filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func removeLinksFile(filepath string) error {
	return os.Remove(filepath)
}
