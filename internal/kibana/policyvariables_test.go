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

func TestSetUseAPMVariable(t *testing.T) {
	defaultVarValue := func(v any) packages.VarValue {
		vv := packages.VarValue{}
		vv.Unpack(v)
		return vv
	}

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
