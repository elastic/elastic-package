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

func defaultVarValue(v any) packages.VarValue {
	vv := packages.VarValue{}
	vv.Unpack(v)
	return vv
}

func TestSetUseAPMVariable(t *testing.T) {
	cases := []struct {
		name              string
		vars              Vars
		variablesToAssign common.MapStr
		wantUseAPM        bool // true = key present and true, false = key present and false, <not set> = key absent
		wantUseAPMPresent bool
		wantUnchanged     bool // when true, vars must be the same map (no new keys, existing keys unchanged)
	}{
		{
			name:              "use_apm already in vars is left unchanged",
			vars:              Vars{"use_apm": {Value: defaultVarValue(false), Type: "boolean"}},
			variablesToAssign: common.MapStr{"use_apm": true},
			wantUseAPMPresent: true,
			wantUseAPM:        false,
			wantUnchanged:     true,
		},
		{
			name:              "no use_apm in variablesToAssign leaves vars unchanged",
			vars:              Vars{},
			variablesToAssign: common.MapStr{},
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
		{
			name:              "use_apm true is added",
			vars:              Vars{},
			variablesToAssign: common.MapStr{"use_apm": true},
			wantUseAPMPresent: true,
			wantUseAPM:        true,
		},
		{
			name:              "use_apm false is added",
			vars:              Vars{},
			variablesToAssign: common.MapStr{"use_apm": false},
			wantUseAPMPresent: true,
			wantUseAPM:        false,
		},
		{
			name:              "use_apm as string true is added",
			vars:              Vars{},
			variablesToAssign: common.MapStr{"use_apm": "true"},
			wantUseAPMPresent: true,
			wantUseAPM:        true,
			wantUnchanged:     false,
		},
		{
			name:              "use_apm as string false is added",
			vars:              Vars{},
			variablesToAssign: common.MapStr{"use_apm": "false"},
			wantUseAPMPresent: true,
			wantUseAPM:        false,
			wantUnchanged:     false,
		},
		{
			name:              "use_apm as unexpected string is not added",
			vars:              Vars{},
			variablesToAssign: common.MapStr{"use_apm": "foo"},
			wantUseAPMPresent: false,
			wantUseAPM:        false,
			wantUnchanged:     true,
		},
		{
			name:              "use_apm as int is not added",
			vars:              Vars{},
			variablesToAssign: common.MapStr{"use_apm": 1},
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
		{
			name:              "other vars are preserved when adding use_apm",
			vars:              Vars{"other": {Value: defaultVarValue("x"), Type: "text"}},
			variablesToAssign: common.MapStr{"use_apm": true},
			wantUseAPMPresent: true,
			wantUseAPM:        true,
		},
		{
			name:              "nil variablesToAssign does not add use_apm",
			vars:              Vars{},
			variablesToAssign: nil,
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Copy vars so we can compare for unchanged when needed
			inputVars := make(Vars, len(c.vars))
			for k, v := range c.vars {
				inputVars[k] = v
			}

			got := SetUseAPMVariable(inputVars, c.variablesToAssign)

			if c.wantUnchanged && len(c.vars) == len(got) {
				for k, v := range c.vars {
					g, ok := got[k]
					require.True(t, ok, "key %q should remain", k)
					assert.Equal(t, v.Value.Value(), g.Value.Value(), "value for %q", k)
				}
			}

			if c.wantUseAPMPresent {
				require.Contains(t, got, "use_apm", "vars should contain use_apm")
				assert.Equal(t, "boolean", got["use_apm"].Type)
				assert.Equal(t, c.wantUseAPM, got["use_apm"].Value.Value())
			} else {
				assert.NotContains(t, got, "use_apm", "vars should not contain use_apm")
			}

			// Original vars must always be preserved
			for k, v := range c.vars {
				require.Contains(t, got, k, "original var %q must be preserved", k)
				assert.Equal(t, v.Value.Value(), got[k].Value.Value(), "value for %q", k)
			}
		})
	}
}

func TestSetDataStreamDatasetVariable(t *testing.T) {
	cases := []struct {
		name               string
		vars               Vars
		variablesToAssign  common.MapStr
		defaultValue       string
		wantDataset        string
		wantDatasetPresent bool
		wantUnchanged      bool
	}{
		{
			name:               "data_stream.dataset already in vars is left unchanged",
			vars:               Vars{"data_stream.dataset": {Value: defaultVarValue("existing"), Type: "text"}},
			variablesToAssign:  common.MapStr{"data_stream.dataset": "overwrite"},
			defaultValue:       "default",
			wantDatasetPresent: true,
			wantDataset:        "existing",
			wantUnchanged:      true,
		},
		{
			name:               "no data_stream.dataset in variablesToAssign uses default",
			vars:               Vars{},
			variablesToAssign:  common.MapStr{},
			defaultValue:       "default.dataset",
			wantDatasetPresent: true,
			wantDataset:        "default.dataset",
		},
		{
			name:               "data_stream.dataset string is set",
			vars:               Vars{},
			variablesToAssign:  common.MapStr{"data_stream.dataset": "custom.dataset"},
			defaultValue:       "default",
			wantDatasetPresent: true,
			wantDataset:        "custom.dataset",
		},
		{
			name:               "data_stream.dataset empty string uses default",
			vars:               Vars{},
			variablesToAssign:  common.MapStr{"data_stream.dataset": ""},
			defaultValue:       "default.dataset",
			wantDatasetPresent: true,
			wantDataset:        "default.dataset",
		},
		{
			name:               "nil variablesToAssign uses default",
			vars:               Vars{},
			variablesToAssign:  nil,
			defaultValue:       "default.from.nil",
			wantDatasetPresent: true,
			wantDataset:        "default.from.nil",
		},
		{
			name:               "other vars are preserved when adding data_stream.dataset",
			vars:               Vars{"other": {Value: defaultVarValue("x"), Type: "text"}},
			variablesToAssign:  common.MapStr{"data_stream.dataset": "my.dataset"},
			defaultValue:       "default",
			wantDatasetPresent: true,
			wantDataset:        "my.dataset",
		},
		{
			name:               "non-string value in variablesToAssign uses default",
			vars:               Vars{},
			variablesToAssign:  common.MapStr{"data_stream.dataset": 123},
			defaultValue:       "default.dataset",
			wantDatasetPresent: true,
			wantDataset:        "default.dataset",
		},
		{
			name:               "defaultValue empty when no override yields empty",
			vars:               Vars{},
			variablesToAssign:  common.MapStr{},
			defaultValue:       "",
			wantDatasetPresent: true,
			wantDataset:        "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inputVars := make(Vars, len(c.vars))
			for k, v := range c.vars {
				inputVars[k] = v
			}

			got := SetDataStreamDatasetVariable(inputVars, c.variablesToAssign, c.defaultValue)

			if c.wantUnchanged && len(c.vars) == len(got) {
				for k, v := range c.vars {
					g, ok := got[k]
					require.True(t, ok, "key %q should remain", k)
					assert.Equal(t, v.Value.Value(), g.Value.Value(), "value for %q", k)
				}
			}

			if c.wantDatasetPresent {
				require.Contains(t, got, "data_stream.dataset", "vars should contain data_stream.dataset")
				assert.Equal(t, "text", got["data_stream.dataset"].Type)
				assert.Equal(t, c.wantDataset, got["data_stream.dataset"].Value.Value())
			} else {
				assert.NotContains(t, got, "data_stream.dataset", "vars should not contain data_stream.dataset")
			}

			for k, v := range c.vars {
				require.Contains(t, got, k, "original var %q must be preserved", k)
				assert.Equal(t, v.Value.Value(), got[k].Value.Value(), "value for %q", k)
			}
		})
	}
}

func TestSetKibanaVariables(t *testing.T) {
	varDef := func(name, typ string, defaultVal any) packages.Variable {
		def := packages.Variable{Name: name, Type: typ}
		if defaultVal != nil {
			vv := defaultVarValue(defaultVal)
			def.Default = &vv
		}
		return def
	}

	cases := []struct {
		name        string
		definitions []packages.Variable
		values      common.MapStr
		wantVars    map[string]any // name -> expected Value(). nil means key must be absent
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
			for name, v := range got {
				wantVal, ok := c.wantVars[name]
				require.True(t, ok, "var %q should not be present", name)
				assert.Equal(t, wantVal, v.Value.Value(), "var %q value", name)
			}
		})
	}
}
