// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"fmt"
	"slices"

	"github.com/elastic/elastic-package/internal/common"
)

func removeDuplicateSharedTags(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	aType, err := object.GetValue("type")
	if err != nil {
		return nil, fmt.Errorf("failed to read type field: %w", err)
	}

	if aType != "tag" {
		return object, nil
	}
	tagName, err := object.GetValue("attributes.name")
	if err != nil {
		return nil, fmt.Errorf("failed to read attributes.name field: %w", err)
	}

	tagNameStr, ok := tagName.(string)
	if !ok {
		return nil, fmt.Errorf("failed to convert tag name to string")
	}
	if slices.Contains(ctx.sharedTags, tagNameStr) {
		// Tag is already in the shared tags list, remove it
		return nil, nil
	}
	return object, nil
}
