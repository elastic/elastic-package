// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProfileConfig(t *testing.T) {
	cases := []struct {
		name     string
		expected string
		found    bool
	}{
		{
			name:     "stack.geoip_dir",
			expected: "/home/foo/Documents/ingest-geoip",
			found:    true,
		},
		{
			name:     "other.empty",
			expected: "",
			found:    true,
		},
		{
			name:     "other.nested",
			expected: "foo",
			found:    true,
		},
		{
			name:     "other.number",
			expected: "42",
			found:    true,
		},
		{
			name:     "other.float",
			expected: "0.12345",
			found:    true,
		},
		{
			name:     "other.bool",
			expected: "false",
			found:    true,
		},
		{
			name:     "stack.apm_server_enabled",
			expected: "true",
			found:    true,
		},
		{
			name:     "stack.logstash_enabled",
			expected: "true",
			found:    true,
		},
		{
			name:  "not.present",
			found: false,
		},
	}

	config, err := loadProfileConfig("_testdata/config.yml")
	require.NoError(t, err)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value, found := config.get(c.name)
			if assert.Equal(t, c.found, found) {
				assert.Equal(t, c.expected, value)
			}
		})
	}
}
