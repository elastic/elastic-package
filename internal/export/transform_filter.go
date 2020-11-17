// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"github.com/elastic/elastic-package/internal/common"
)

func filterUnsupportedTypes(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	aType, _ := object.GetValue("type")
	switch aType {
	case "index-pattern": // unsupported types
		return nil, nil
	default:
		return object, nil
	}
}
