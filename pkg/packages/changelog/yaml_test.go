// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package changelog

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchYAML(t *testing.T) {
	cases := []struct {
		title    string
		original string
		expected string
		patch    Revision
	}{
		{
			title:    "Change in current version",
			original: "testdata/changelog-one.yml",
			expected: "testdata/changelog-one-patch-same-version.yml",
			patch: Revision{
				Version: "1.0.0",
				Changes: []Entry{
					{
						Description: "One change",
						Type:        "enhancement",
						Link:        "http://github.com/elastic/elastic-package",
					},
				},
			},
		},
		{
			title:    "Change in next major",
			original: "testdata/changelog-one.yml",
			expected: "testdata/changelog-one-patch-next-major.yml",
			patch: Revision{
				Version: "2.0.0",
				Changes: []Entry{
					{
						Description: "One change",
						Type:        "enhancement",
						Link:        "http://github.com/elastic/elastic-package",
					},
				},
			},
		},
		{
			title:    "Multiple changes",
			original: "testdata/changelog-one.yml",
			expected: "testdata/changelog-one-patch-multiple.yml",
			patch: Revision{
				Version: "1.0.0",
				Changes: []Entry{
					{
						Description: "One change",
						Type:        "enhancement",
						Link:        "http://github.com/elastic/elastic-package",
					},
					{
						Description: "Other change",
						Type:        "enhancement",
						Link:        "http://github.com/elastic/elastic-package",
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			d, err := os.ReadFile(c.original)
			require.NoError(t, err)

			result, err := PatchYAML(d, c.patch)
			require.NoError(t, err)

			expected, err := os.ReadFile(c.expected)
			if errors.Is(err, os.ErrNotExist) {
				err := os.WriteFile(c.expected, result, 0644)
				require.NoError(t, err)
				t.Skip("file generated, run again")
			}
			require.NoError(t, err)

			assert.Equal(t, string(expected), string(result))
		})
	}
}

func TestManifestVersion(t *testing.T) {
	manifest := "name: test\nversion: 1.0.0\ncategories:\n  - custom\n"
	expected := "name: test\nversion: 1.1.0\ncategories:\n  - custom\n"

	result, err := SetManifestVersion([]byte(manifest), "1.1.0")
	require.NoError(t, err)

	assert.Equal(t, string(expected), string(result))
}
