// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"fmt"
	"log"
	"strings"

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
	log.Printf("Removing tags")
	aId, err := object.GetValue("id")
	if err == common.ErrKeyNotFound {
		return object, nil
	}

	switch aId {
	case fmt.Sprintf("fleet-pkg-%s-default", ctx.packageName):
		return nil, nil
	case "fleet-managed-default":
		return nil, nil
	}
	return object, nil
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

		log.Printf("Id tag .> %s", aId)
		switch aId {
		case fmt.Sprintf("fleet-pkg-%s-default", ctx.packageName):
			log.Printf("Matched %s", fmt.Sprintf("fleet-pkg-%s-default", ctx.packageName))
			continue
		case "fleet-managed-default":
			log.Printf("Matched fleet-managed-default")
			continue
		case fmt.Sprintf("%s-fleet-pkg-%s-default", ctx.packageName, ctx.packageName):
			log.Printf("Matched %s", fmt.Sprintf("%s-fleet-pkg-%s-default", ctx.packageName, ctx.packageName))
			continue
		case fmt.Sprintf("%s-fleet-managed-default", ctx.packageName):
			log.Printf("Matched %s", fmt.Sprintf("%s-fleet-managed-default", ctx.packageName))
			continue
		}
		continue

		aName, ok := reference["name"]
		if !ok {
			continue
		}
		if aName == "tag-ref-fleet-managed-default" {
			continue
		}
		if strings.HasPrefix(aName.(string), "tag-ref-fleet-pkg") {
			continue
		}
		newReferences = append(newReferences, r)
	}
	return newReferences, nil
}
