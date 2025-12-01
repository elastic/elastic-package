// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "empty key",
			apiKey:   "",
			expected: "",
		},
		{
			name:     "short key",
			apiKey:   "abc123",
			expected: "******",
		},
		{
			name:     "exactly 12 chars",
			apiKey:   "abcdef123456",
			expected: "************",
		},
		{
			name:     "long key",
			apiKey:   "sk-proj-abc123xyz789",
			expected: "****************z789",
		},
		{
			name:     "13 chars shows last 4",
			apiKey:   "1234567890123",
			expected: "*********0123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskAPIKey(tt.apiKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}
