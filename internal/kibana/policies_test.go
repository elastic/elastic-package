// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
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
			"ssl": Var{Type: "yaml", Value: sslValue, fromUser: true},
		}

		m := vars.ToMapStr()

		require.NotNil(t, m)
		assert.Equal(t, yamlStr, m["ssl"])
	})

	t.Run("yaml type map is serialized to YAML string", func(t *testing.T) {
		// When a test config writes a yaml-type var as a YAML map (without the |
		// block scalar), go-ucfg parses it as map[string]interface{}. The
		// simplified Fleet API only accepts strings for yaml-type vars, so
		// ToMapStr must serialize the map to a YAML string.
		var sslValue packages.VarValue
		require.NoError(t, sslValue.Unpack(map[string]interface{}{"verification_mode": "none"}))
		vars := Vars{
			"ssl": Var{Type: "yaml", Value: sslValue, fromUser: true},
		}

		m := vars.ToMapStr()

		require.NotNil(t, m)
		assert.Equal(t, "verification_mode: none", m["ssl"])
	})

	t.Run("yaml type comment-only string is passed through as-is", func(t *testing.T) {
		// Comment-only YAML strings provided by the user are passed through unchanged.
		commentOnly := "#- tz_short: AEST\n#  tz_long: Australia/Sydney\n"
		var tzValue packages.VarValue
		require.NoError(t, tzValue.Unpack(commentOnly))
		vars := Vars{
			"tz_map": Var{Type: "yaml", Value: tzValue, fromUser: true},
		}

		m := vars.ToMapStr()

		require.NotNil(t, m)
		assert.Equal(t, commentOnly, m["tz_map"])
	})

	t.Run("non-yaml type is passed through as-is", func(t *testing.T) {
		var val packages.VarValue
		require.NoError(t, val.Unpack("http://localhost:8080"))
		vars := Vars{
			"url": Var{Type: "text", Value: val, fromUser: true},
		}

		m := vars.ToMapStr()

		require.NotNil(t, m)
		assert.Equal(t, "http://localhost:8080", m["url"])
	})

	t.Run("nil yaml value is passed through as nil", func(t *testing.T) {
		vars := Vars{
			"ssl": Var{Type: "yaml", Value: packages.VarValue{}, fromUser: true},
		}

		m := vars.ToMapStr()

		require.NotNil(t, m)
		assert.Nil(t, m["ssl"])
	})

	t.Run("manifest default is excluded from ToMapStr", func(t *testing.T) {
		// Vars with fromUser==false (manifest defaults) must not appear in simplified
		// API requests; the server applies them when compiling templates.
		var val packages.VarValue
		require.NoError(t, val.Unpack("UTC"))
		vars := Vars{
			"tz_offset": Var{Type: "text", Value: val},
		}
		assert.Nil(t, vars.ToMapStr())
	})

	t.Run("empty vars returns nil", func(t *testing.T) {
		assert.Nil(t, Vars{}.ToMapStr())
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

	policy := PackagePolicy{
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
	policy.Package.Name = "apache"
	policy.Package.Version = "1.0.0"

	legacy := policy.toLegacy()

	assert.Equal(t, "test-policy", legacy.Name)
	assert.Equal(t, "default", legacy.Namespace)
	assert.Equal(t, "agent-policy-id", legacy.PolicyID)
	assert.True(t, legacy.Enabled, "legacy policy must have enabled=true")
	assert.Equal(t, "apache", legacy.Package.Name)

	require.Len(t, legacy.Inputs, 1)

	// Find and verify the enabled input.
	enabledInput := &legacy.Inputs[0]
	require.Equal(t, "apache/metrics", enabledInput.Type, "only the enabled apache/metrics input should be present")
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

func TestSetKibanaVariables(t *testing.T) {
	varDef := func(name, typ string, defaultVal any) packages.Variable {
		def := packages.Variable{Name: name, Type: typ}
		if defaultVal != nil {
			vv := packages.VarValue{}
			vv.Unpack(defaultVal)
			def.Default = &vv
		}
		return def
	}

	cases := []struct {
		name        string
		definitions []packages.Variable
		values      common.MapStr
		wantVars    map[string]any // name -> expected Value(). Only vars expected in result.
	}{
		{
			name:        "empty definitions returns empty vars",
			definitions: nil,
			values:      common.MapStr{"any": "value"},
			wantVars:    map[string]any{},
		},
		{
			name:        "definition with default and no values uses default",
			definitions: []packages.Variable{varDef("host", "text", "localhost")},
			values:      common.MapStr{},
			wantVars:    map[string]any{"host": "localhost"},
		},
		{
			name:        "definition with default overridden by values",
			definitions: []packages.Variable{varDef("host", "text", "localhost")},
			values:      common.MapStr{"host": "elastic.co"},
			wantVars:    map[string]any{"host": "elastic.co"},
		},
		{
			name:        "definition with no default and no value is omitted",
			definitions: []packages.Variable{varDef("optional", "text", nil)},
			values:      common.MapStr{},
			wantVars:    map[string]any{},
		},
		{
			name:        "definition with no default but value in values is included",
			definitions: []packages.Variable{varDef("optional", "text", nil)},
			values:      common.MapStr{"optional": "set"},
			wantVars:    map[string]any{"optional": "set"},
		},
		{
			name: "nil values uses defaults only",
			definitions: []packages.Variable{
				varDef("a", "text", "default_a"),
				varDef("b", "text", nil),
			},
			values:   nil,
			wantVars: map[string]any{"a": "default_a"},
		},
		{
			name: "multiple definitions mix default and override",
			definitions: []packages.Variable{
				varDef("host", "text", "localhost"),
				varDef("port", "integer", 9200),
				varDef("optional", "text", nil),
			},
			values: common.MapStr{"port": 9300},
			wantVars: map[string]any{
				"host": "localhost",
				"port": 9300,
			},
		},
		{
			name:        "boolean and list types preserved",
			definitions: []packages.Variable{varDef("enabled", "bool", true), varDef("hosts", "text", []any{"a", "b"})},
			values:      common.MapStr{},
			wantVars: map[string]any{
				"enabled": true,
				"hosts":   []any{"a", "b"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SetKibanaVariables(c.definitions, c.values)

			assert.Len(t, got, len(c.wantVars), "number of vars")
			for name, wantVal := range c.wantVars {
				require.Contains(t, got, name, "var %q should be present", name)
				assert.Equal(t, wantVal, got[name].Value.Value(), "var %q value", name)
			}
			for name := range got {
				_, ok := c.wantVars[name]
				assert.True(t, ok, "unexpected var %q in result", name)
			}
		})
	}
}
