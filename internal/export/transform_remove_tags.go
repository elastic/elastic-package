// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"fmt"
	"log"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
)

func removeFleetManagedTags(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	aType, err := object.GetValue("type")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read type field")
	}

	if aType == "dashboard" {
		return removeTagsFromDashboard(ctx, object)
	}

	if aType == "tag" {
		return removeTagObjects(ctx, object)
	}

	return object, nil
}

func removeTagsFromDashboard(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	log.Printf("Removing tags from dashboard")
	references, err := object.GetValue("references")
	if err == common.ErrKeyNotFound {
		return object, nil
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to read references field")
	}

	newReferences, err := filterOutFleetManagedTags(ctx, references.([]interface{}))

	_, err = object.Put("references", newReferences)
	if err != nil {
		return nil, errors.Wrapf(err, "can't update references")
	}

	return object, nil
}

func removeTagObjects(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	log.Printf("Removing tags objects")
	aId, err := object.GetValue("id")
	if err == common.ErrKeyNotFound {
		return object, nil
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to read id field")
	}

	aIdString, ok := aId.(string)
	if !ok {
		return nil, errors.Wrap(err, "failed to read id string")
	}

	if isTagFleetManaged(aIdString, ctx.packageName) {
		log.Printf("1. Removing fleet tag object: %s", aId)
		return nil, nil
	}
	log.Printf("2. Adding tag: %s", aId)
	return object, nil
}

func isTagFleetManaged(aId, packageName string) bool {
	var empty interface{}
	fleetManagedTags := map[string]interface{}{
		fmt.Sprintf("fleet-pkg-%s-default", packageName):                 empty,
		"fleet-managed-default":                                          empty,
		fmt.Sprintf("%s-fleet-pkg-%s-default", packageName, packageName): empty,
		fmt.Sprintf("%s-fleet-managed-default", packageName):             empty,
	}

	_, ok := fleetManagedTags[aId]
	return ok
}

func filterOutFleetManagedTags(ctx *transformationContext, references []interface{}) ([]interface{}, error) {
	newReferences := make([]interface{}, 0)
	for _, r := range references {
		reference := r.(map[string]interface{})

		aType, ok := reference["type"]
		if !ok {
			continue
		}
		if aType != "tag" {
			newReferences = append(newReferences, r)
			continue
		}

		aId, ok := reference["id"]
		if !ok {
			newReferences = append(newReferences, r)
			continue
		}

		aIdString, ok := aId.(string)
		if !ok {
			return nil, errors.New("failed to read id string")
		}
		log.Printf("Id tag .> %s", aIdString)
		if isTagFleetManaged(aIdString, ctx.packageName) {
			continue
		}
		newReferences = append(newReferences, r)
	}
	return newReferences, nil
}
