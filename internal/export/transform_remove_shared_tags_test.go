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
		sharedTags     []string
		inputObject    common.MapStr
		expectedObject common.MapStr
	}{
		{
			name:       "Tag already in shared tags",
			sharedTags: []string{"shared-tag-1", "shared-tag-2"},
			inputObject: map[string]interface{}{
				"type":       "tag",
				"attributes": map[string]interface{}{"name": "shared-tag-1"},
			},
			expectedObject: nil,
		},
		{
			name:       "Tag not in shared tags",
			sharedTags: []string{"shared-tag-1", "shared-tag-2"},
			inputObject: map[string]interface{}{
				"type":       "tag",
				"attributes": map[string]interface{}{"name": "unique-tag"},
			},
			expectedObject: map[string]interface{}{
				"type":       "tag",
				"attributes": map[string]interface{}{"name": "unique-tag"},
			},
		},
		{
			name:       "Non-tag object",
			sharedTags: []string{"shared-tag-1", "shared-tag-2"},
			inputObject: map[string]interface{}{
				"type":       "dashboard",
				"attributes": map[string]interface{}{"title": "My Dashboard"},
			},
			expectedObject: map[string]interface{}{
				"type":       "dashboard",
				"attributes": map[string]interface{}{"title": "My Dashboard"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &transformationContext{
				sharedTags: tt.sharedTags,
			}
			inputMapStr := tt.inputObject
			result, err := removeDuplicateSharedTags(ctx, inputMapStr)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedObject, result)
		})
	}
}
