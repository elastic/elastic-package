// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigWithSettings(t *testing.T) {
	example, err := static.ReadFile("_static/config.yml.example")
	require.NoError(t, err)
	existing, err := os.ReadFile("testdata/config.yml")
	require.NoError(t, err)

	cases := []struct {
		name     string
		content  []byte
		settings map[string]string
		expected string
	}{
		{
			name: "no configuration",
			settings: map[string]string{
				"stack.logstash_enabled":     "true",
				"stack.elastic_subscription": "basic",
			},
			expected: "stack.elastic_subscription: basic\n" +
				"stack.logstash_enabled: \"true\"\n",
		},
		{
			name:    "configuration without settings",
			content: example,
			settings: map[string]string{
				"stack.elastic_subscription": "basic",
			},
			expected: string(example) +
				"stack.elastic_subscription: basic\n",
		},
		{
			name:    "configuration with settings",
			content: existing,
			settings: map[string]string{
				// Replaces the value of a setting.
				"stack.elastic_subscription": "trial",
				// Takes precedence over the value of a nested setting.
				"other.nested": "bar",
				// New setting.
				"stack.self_monitor_enabled": "true",
			},
			expected: strings.Replace(string(existing), "stack.elastic_subscription: basic", "stack.elastic_subscription: trial", 1) + "\n" +
				"other.nested: bar\n" +
				"stack.self_monitor_enabled: \"true\"\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			content, err := configWithSettings(c.content, c.settings)
			require.NoError(t, err)
			assert.Equal(t, c.expected, string(content))

			path := filepath.Join(t.TempDir(), PackageProfileConfigFile)
			err = os.WriteFile(path, content, 0o644)
			require.NoError(t, err)
			config, err := loadProfileConfig(path)
			require.NoError(t, err)
			for name, expected := range c.settings {
				value, found := config.get(name)
				if assert.True(t, found, name) {
					assert.Equal(t, expected, value, name)
				}
			}
		})
	}
}

func TestConfigWithSettingsListsAndMappings(t *testing.T) {
	existing := []byte("stack.agent.ports:\n- 127.0.0.1:1514:1514/udp\n")
	settings := map[string]string{
		// Replaces a list.
		"stack.agent.ports": "[127.0.0.1:1515:1515/udp, 127.0.0.1:1516:1516/tcp]",
		// New mapping.
		"llm.tracing.headers": `{Authorization: "Bearer token"}`,
	}

	content, err := configWithSettings(existing, settings)
	require.NoError(t, err)
	assert.Equal(t, "stack.agent.ports: [127.0.0.1:1515:1515/udp, 127.0.0.1:1516:1516/tcp]\n"+
		"llm.tracing.headers: {Authorization: \"Bearer token\"}\n", string(content))

	path := filepath.Join(t.TempDir(), PackageProfileConfigFile)
	err = os.WriteFile(path, content, 0o644)
	require.NoError(t, err)
	config, err := loadProfileConfig(path)
	require.NoError(t, err)

	var ports []string
	require.NoError(t, config.Decode("stack.agent.ports", &ports))
	assert.Equal(t, []string{"127.0.0.1:1515:1515/udp", "127.0.0.1:1516:1516/tcp"}, ports)

	var headers map[string]string
	require.NoError(t, config.Decode("llm.tracing.headers", &headers))
	assert.Equal(t, map[string]string{"Authorization": "Bearer token"}, headers)

	for _, invalid := range []string{"[a, b", "{a: b}}", "[a]\n---\n[b]"} {
		_, err := configWithSettings(nil, map[string]string{"stack.agent.ports": invalid})
		assert.Error(t, err, invalid)
	}
}

func TestConfigWithSettingsUnsupportedContent(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "list", content: "- stack.logstash_enabled: true\n"},
		{name: "flow mapping", content: "{stack.logstash_enabled: true}\n"},
		{name: "multiple documents", content: "stack.logstash_enabled: true\n---\nstack.apm_enabled: true\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := configWithSettings([]byte(c.content), map[string]string{"stack.elastic_subscription": "basic"})
			assert.Error(t, err)
		})
	}
}
