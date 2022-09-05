// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderUrl(t *testing.T) {
	cases := []struct {
		title    string
		defs     linkMap
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
			"intro",
			false,
			"http://package-spec.test/intro",
		},
		{
			"key not exist",
			linkMap{
				map[string]string{
					"intro": "http://package-spec.test/intro",
					"docs":  "http://package-spec.test/docs",
				},
			},
			"notexist",
			true,
			"",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			output, err := c.defs.RenderLink(c.key, linkOptions{})
			if c.errors {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.expected, output)
		})
	}
}

func TestRenderLInk(t *testing.T) {
	cases := []struct {
		title    string
		defs     linkMap
		key      string
		link     string
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
			"intro",
			"Introduction",
			false,
			"[Introduction](http://package-spec.test/intro)",
		},
		{
			"key not exist",
			linkMap{
				map[string]string{
					"intro": "http://package-spec.test/intro",
					"docs":  "http://package-spec.test/docs",
				},
			},
			"notexist",
			"Not Exist",
			true,
			"",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			output, err := c.defs.RenderLink(c.key, linkOptions{caption: c.link})
			if c.errors {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.expected, output)
		})
	}
}
