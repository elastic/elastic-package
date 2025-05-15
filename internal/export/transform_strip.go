// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/common"
)

func stripObjectProperties(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	err := object.Delete("namespaces")
	if err != nil && err != common.ErrKeyNotFound {
		return nil, fmt.Errorf("removing field \"namespaces\" failed: %w", err)
	}

	err = object.Delete("updated_at")
	if err != nil && err != common.ErrKeyNotFound {
		return nil, fmt.Errorf("removing field \"updated_at\" failed: %w", err)
	}

	err = object.Delete("version")
	if err != nil && err != common.ErrKeyNotFound {
		return nil, fmt.Errorf("removing field \"version\" failed: %w", err)
	}

	err = object.Delete("managed")
	if err != nil && err != common.ErrKeyNotFound {
		return nil, fmt.Errorf("removing field \"managed\" failed: %w", err)
	}

	return object, nil
}
