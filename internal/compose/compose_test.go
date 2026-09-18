// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package compose

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestIntOrStringYaml(t *testing.T) {
	cases := []struct {
		yaml     string
		expected int
	}{
		{`"9200"`, 9200},
		{`'9200'`, 9200},
		{`9200`, 9200},
	}

	for _, c := range cases {
		t.Run(c.yaml, func(t *testing.T) {
			var n intOrStringYaml
			err := yaml.Unmarshal([]byte(c.yaml), &n)
			require.NoError(t, err)
			assert.Equal(t, c.expected, int(n))
		})
	}
}

func TestComposeProgressOutput(t *testing.T) {
	cases := []struct {
		version  string
		expected string
	}{
		// --progress does not exist before 2.19.0.
		{"2.18.1", ""},
		{"2.19.0", "plain"},
		{"2.40.0", "plain"},
		// Bake-based builds require a console when they believe they have one; plain
		// progress is what keeps them working under our pseudo-terminal.
		{"5.5.1", "plain"},
	}

	for _, c := range cases {
		t.Run(c.version, func(t *testing.T) {
			assert.Equal(t, c.expected, composeProgressOutput(semver.MustParse(c.version)))
		})
	}
}
