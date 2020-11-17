// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
)

type objectTransformer struct {
	ctx        *transformationContext
	transforms []func(*transformationContext, common.MapStr) (common.MapStr, error)
}

type transformationContext struct {
	packageName string
}

func newObjectTransformer() *objectTransformer {
	return new(objectTransformer)
}

func (ot *objectTransformer) transform(objects []common.MapStr) ([]common.MapStr, error) {
	var decoded []common.MapStr
	var err error

	for _, object := range objects {
		for _, fn := range ot.transforms {
			if object == nil {
				continue
			}

			object, err = fn(ot.ctx, object)
			if err != nil {
				id, _ := object.GetValue("id")
				return nil, errors.Wrapf(err, "object transformation failed (ID: %s)", id)
			}
		}

		if object != nil {
			decoded = append(decoded, object)
		}
	}
	return decoded, nil
}

func (ot *objectTransformer) withContext(ctx *transformationContext) *objectTransformer {
	ot.ctx = ctx
	return ot
}

func (ot *objectTransformer) withTransforms(transforms ...func(*transformationContext, common.MapStr) (common.MapStr, error)) *objectTransformer {
	ot.transforms = transforms
	return ot
}
