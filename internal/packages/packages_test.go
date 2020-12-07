// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVarValue_MarshalJSON(t *testing.T) {
	t.Run("null", func(t *testing.T) {
		var vv VarValue
		data, err := json.Marshal(vv)
		require.NoError(t, err)
		assert.Equal(t, string(data), "null")
	})

	t.Run("scalar", func(t *testing.T) {
		vv := VarValue{
			scalar: "hello",
		}
		data, err := json.Marshal(vv)
		require.NoError(t, err)
		assert.Equal(t, string(data), `"hello"`)
	})

	t.Run("array", func(t *testing.T) {
		vv := VarValue{
			list: []interface{}{
				"hello",
				"world",
			},
		}

		data, err := json.Marshal(vv)
		require.NoError(t, err)
		assert.Equal(t, string(data), `["hello","world"]`)
	})
}
