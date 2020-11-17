// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
)

func stripObjectProperties(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	err := object.Delete("namespaces")
	if err != nil && err != common.ErrKeyNotFound {
		return nil, errors.Wrapf(err, "removing field \"namespaces\" failed")
	}

	err = object.Delete("updated_at")
	if err != nil && err != common.ErrKeyNotFound {
		return nil, errors.Wrapf(err, "removing field \"updated_at\" failed")
	}

	err = object.Delete("version")
	if err != nil && err != common.ErrKeyNotFound {
		return nil, errors.Wrapf(err, "removing field \"version\" failed")
	}
	return object, nil
}
