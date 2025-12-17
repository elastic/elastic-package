// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"fmt"

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
	tagId, err := object.GetValue("id")
	if err != nil {
		return nil, fmt.Errorf("failed to read id field: %w", err)
	}

	tagIdStr, ok := tagId.(string)
	if !ok {
		return nil, fmt.Errorf("failed to convert tag id to string")
	}
	if isSharedTag(tagIdStr, ctx.packageName) {
		return nil, nil
	}
	return object, nil
}
