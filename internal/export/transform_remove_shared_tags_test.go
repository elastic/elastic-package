// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
)

func TestRemoveDuplicateSharedTags(t *testing.T) {
	tests := []struct {
		name           string
		inputObject    common.MapStr
		expectedObject common.MapStr
	}{
		{
			name: "Is default shared tag",
			inputObject: map[string]interface{}{
				"type": "tag",
				"id":   "fleet-shared-tag-system-default",
			},
			expectedObject: nil,
		},
		{
			name: "Is security solution shared tag",
			inputObject: map[string]interface{}{
				"type": "tag",
				"id":   "system-security-solution-default",
			},
			expectedObject: nil,
		},
		{
			name: "Is not shared tag",
			inputObject: map[string]interface{}{
				"type": "tag",
				"id":   "unique-tag",
			},
			expectedObject: map[string]interface{}{
				"type": "tag",
				"id":   "unique-tag",
			},
		},
		{
			name: "Non-tag object",
			inputObject: map[string]interface{}{
				"type": "dashboard",
				"id":   "My Dashboard",
			},
			expectedObject: map[string]interface{}{
				"type": "dashboard",
				"id":   "My Dashboard",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &transformationContext{}
			inputMapStr := tt.inputObject
			result, err := removeDuplicateSharedTags(ctx, inputMapStr)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedObject, result)
		})
	}
}
