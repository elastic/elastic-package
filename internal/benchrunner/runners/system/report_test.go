// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSecretVars(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		vars        map[string]interface{}
		secretNames map[string]bool
		expected    map[string]interface{}
	}{
		{
			name: "masks secret vars and preserves non-secret vars",
			vars: map[string]interface{}{
				"api_key":   "super-secret-value",
				"host":      "localhost",
				"retries":   3,
				"tls":       true,
				"namespace": "default",
			},
			secretNames: map[string]bool{
				"api_key": true,
			},
			expected: map[string]interface{}{
				"api_key":   "xxxx",
				"host":      "localhost",
				"retries":   3,
				"tls":       true,
				"namespace": "default",
			},
		},
		{
			name: "returns vars unchanged when there are no secret names",
			vars: map[string]interface{}{
				"host": "localhost",
			},
			secretNames: map[string]bool{},
			expected: map[string]interface{}{
				"host": "localhost",
			},
		},
		{
			name: "returns empty vars map when vars are empty",
			vars: map[string]interface{}{},
			secretNames: map[string]bool{
				"api_key": true,
			},
			expected: map[string]interface{}{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original := make(map[string]interface{}, len(tc.vars))
			for k, v := range tc.vars {
				original[k] = v
			}

			actual := maskSecretVars(tc.vars, tc.secretNames)

			assert.Equal(t, tc.expected, actual)
			assert.Equal(t, original, tc.vars)
		})
	}
}
