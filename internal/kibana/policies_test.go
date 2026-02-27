// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"testing"

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
