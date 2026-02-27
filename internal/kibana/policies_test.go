// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestVarsToMapStr(t *testing.T) {
	t.Run("yaml type already a string is passed through as-is", func(t *testing.T) {
		// When the value in the test config is already written as a YAML string
		// (e.g. ssl: |- ... ), it must not be double-encoded.
		yamlStr := "verification_mode: none\ncertificate: /etc/pki/cert.pem\n"
		var sslValue packages.VarValue
		require.NoError(t, sslValue.Unpack(yamlStr))
		vars := Vars{
			"ssl": Var{Type: "yaml", Value: sslValue},
		}

		m := vars.ToMapStr()

		require.NotNil(t, m)
		assert.Equal(t, yamlStr, m["ssl"])
	})

	t.Run("yaml type is serialized as YAML string", func(t *testing.T) {
		var sslValue packages.VarValue
		require.NoError(t, sslValue.Unpack(map[string]interface{}{
			"verification_mode": "none",
		}))
		vars := Vars{
			"ssl": Var{Type: "yaml", Value: sslValue},
		}

		m := vars.ToMapStr()

		require.NotNil(t, m)
		v, ok := m["ssl"]
		require.True(t, ok)
		s, ok := v.(string)
		require.True(t, ok, "expected string for yaml var, got %T", v)
		assert.Contains(t, s, "verification_mode: none")
	})

	t.Run("non-yaml type is passed through as-is", func(t *testing.T) {
		var val packages.VarValue
		require.NoError(t, val.Unpack("http://localhost:8080"))
		vars := Vars{
			"url": Var{Type: "text", Value: val},
		}

		m := vars.ToMapStr()

		require.NotNil(t, m)
		assert.Equal(t, "http://localhost:8080", m["url"])
	})

	t.Run("nil yaml value is passed through as nil", func(t *testing.T) {
		vars := Vars{
			"ssl": Var{Type: "yaml", Value: packages.VarValue{}},
		}

		m := vars.ToMapStr()

		require.NotNil(t, m)
		assert.Nil(t, m["ssl"])
	})

	t.Run("empty vars returns nil", func(t *testing.T) {
		assert.Nil(t, Vars{}.ToMapStr())
	})
}

func TestSupportsSimplifiedPackagePolicyAPI(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"7.14.1", false},
		{"7.15.2", false},
		{"7.16.0", true},
		{"7.17.0", true},
		{"8.0.0", true},
		{"8.15.3", true},
	}
	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			c := &Client{semver: semver.MustParse(tc.version)}
			assert.Equal(t, tc.want, c.supportsSimplifiedPackagePolicyAPI())
		})
	}

	t.Run("managed Kibana (no version) returns true", func(t *testing.T) {
		c := &Client{semver: nil}
		assert.True(t, c.supportsSimplifiedPackagePolicyAPI())
	})
}

func TestToLegacyPackagePolicy(t *testing.T) {
	// Build a PackagePolicy as BuildIntegrationPackagePolicy would produce it,
	// then verify the legacy conversion has the right structure.
	var periodVal, hostVal packages.VarValue
	require.NoError(t, periodVal.Unpack("30s"))
	require.NoError(t, hostVal.Unpack("http://localhost:8080"))

	streamVars := Vars{
		"period": Var{Type: "text", Value: periodVal},
	}
	inputVars := Vars{
		"hosts": Var{Type: "text", Value: hostVal},
	}

	pp := PackagePolicy{
		Name:      "test-policy",
		Namespace: "default",
		PolicyID:  "agent-policy-id",
		Inputs: map[string]PackagePolicyInput{
			"apache-apache/metrics": {
				Enabled:        true,
				Vars:           inputVars.ToMapStr(),
				legacyVars:     inputVars,
				inputType:      "apache/metrics",
				policyTemplate: "apache",
				Streams: map[string]PackagePolicyStream{
					"apache.status": {
						Enabled:           true,
						Vars:              streamVars.ToMapStr(),
						legacyVars:        streamVars,
						dataStreamType:    "metrics",
						dataStreamDataset: "apache.status",
					},
				},
			},
			"apache-logfile": {
				Enabled:        false,
				inputType:      "logfile",
				policyTemplate: "apache",
			},
		},
	}
	pp.Package.Name = "apache"
	pp.Package.Version = "1.0.0"

	legacy := toLegacyPackagePolicy(pp)

	assert.Equal(t, "test-policy", legacy.Name)
	assert.Equal(t, "default", legacy.Namespace)
	assert.Equal(t, "agent-policy-id", legacy.PolicyID)
	assert.True(t, legacy.Enabled, "legacy policy must have enabled=true")
	assert.Equal(t, "apache", legacy.Package.Name)

	require.Len(t, legacy.Inputs, 2)

	// Find and verify the enabled input.
	var enabledInput *legacyInput
	for i := range legacy.Inputs {
		if legacy.Inputs[i].Type == "apache/metrics" {
			enabledInput = &legacy.Inputs[i]
		}
	}
	require.NotNil(t, enabledInput, "apache/metrics input not found in legacy inputs")
	assert.Equal(t, "apache", enabledInput.PolicyTemplate)
	assert.True(t, enabledInput.Enabled)

	// Verify input-level vars use {value, type} wrappers.
	require.Contains(t, enabledInput.Vars, "hosts")
	assert.Equal(t, "http://localhost:8080", enabledInput.Vars["hosts"].Value.Value())
	assert.Equal(t, "text", enabledInput.Vars["hosts"].Type)

	// Verify stream.
	require.Len(t, enabledInput.Streams, 1)
	assert.Equal(t, "metrics", enabledInput.Streams[0].DataStream.Type)
	assert.Equal(t, "apache.status", enabledInput.Streams[0].DataStream.Dataset)
	require.Contains(t, enabledInput.Streams[0].Vars, "period")
	assert.Equal(t, "30s", enabledInput.Streams[0].Vars["period"].Value.Value())
	assert.Equal(t, "text", enabledInput.Streams[0].Vars["period"].Type)
}
