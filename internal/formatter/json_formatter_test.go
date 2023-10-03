// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter_test

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/formatter"
)

func TestJSONFormatterFormat(t *testing.T) {
	cases := []struct {
		title    string
		version  *semver.Version
		content  string
		expected string
		valid    bool
	}{
		{
			title:   "invalid json 2.0",
			version: semver.MustParse("2.0.0"),
			content: `{"foo":}`,
			valid:   false,
		},
		{
			title:   "invalid json 3.0",
			version: semver.MustParse("3.0.0"),
			content: `{"foo":}`,
			valid:   false,
		},
		{
			title:   "encode html in old versions",
			version: semver.MustParse("2.0.0"),
			content: `{"a": "<script></script>"}`,
			expected: `{
    "a": "\u003cscript\u003e\u003c/script\u003e"
}`,
			valid: true,
		},
		{
			title:   "don't encode html since 2.12.0",
			version: semver.MustParse("2.12.0"),
			content: `{"a": "<script></script>"}`,
			expected: `{
    "a": "<script></script>"
}`,
			valid: true,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			jsonFormatter := formatter.JSONFormatterBuilder(*c.version)
			formatted, equal, err := jsonFormatter.Format([]byte(c.content))
			if !c.valid {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, c.expected, string(formatted))
			assert.Equal(t, c.content == c.expected, equal)
		})
	}
}

func TestJSONFormatterEncode(t *testing.T) {
	cases := []struct {
		title    string
		version  *semver.Version
		object   any
		expected string
	}{
		{
			title:   "encode html in old versions",
			version: semver.MustParse("2.0.0"),
			object:  map[string]any{"a": "<script></script>"},
			expected: `{
    "a": "\u003cscript\u003e\u003c/script\u003e"
}`,
		},
		{
			title:   "don't encode html since 2.12.0",
			version: semver.MustParse("2.12.0"),
			object:  map[string]any{"a": "<script></script>"},
			expected: `{
    "a": "<script></script>"
}`,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			jsonFormatter := formatter.JSONFormatterBuilder(*c.version)
			formatted, err := jsonFormatter.Encode(c.object)
			require.NoError(t, err)
			assert.Equal(t, c.expected, string(formatted))
		})
	}
}
