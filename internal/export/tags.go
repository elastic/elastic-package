// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/common"
)

// removeFleetTags removes fleet managed and shared tags from the given Kibana object.
func removeFleetTags(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	aType, err := object.GetValue("type")
	if err != nil {
		return nil, fmt.Errorf("failed to read type field: %w", err)
	}

	if aType == "tag" {
		return removeTagObjects(ctx, object)
	}

	return removeTagReferences(ctx, object)
}

func removeTagReferences(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	references, err := object.GetValue("references")
	if err == common.ErrKeyNotFound {
		return object, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read references field: %w", err)
	}

	newReferences, err := filterOutFleetManagedTags(ctx, references.([]interface{}))
	if err != nil {
		return nil, err
	}

	newReferences, err = filterOutSharedTags(ctx, newReferences)
	if err != nil {
		return nil, err
	}

	_, err = object.Put("references", newReferences)
	if err != nil {
		return nil, fmt.Errorf("can't update references: %w", err)
	}

	return object, nil
}

func removeTagObjects(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	aId, err := object.GetValue("id")
	if err == common.ErrKeyNotFound {
		return object, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read id field: %w", err)
	}

	aIdString, ok := aId.(string)
	if !ok {
		return nil, fmt.Errorf("failed to assert id as a string: %v", aId)
	}

	if isTagFleetManaged(aIdString, ctx.packageName) {
		return nil, nil
	}
	if isSharedTag(aIdString, ctx.packageName) {
		return nil, nil
	}
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
			return nil, fmt.Errorf("failed to assert id as a string: %v", aId)
		}
		if isTagFleetManaged(aIdString, ctx.packageName) {
			continue
		}
		newReferences = append(newReferences, r)
	}
	return newReferences, nil
}

// isSharedTag checks if the given tag ID is a shared tag for the specified package.
// Shared tags ids are created by fleet form the tags.yml file in the package.
// https://github.com/elastic/kibana/blob/5385f96a132114362b2542e6a44c96a697b66c28/x-pack/platform/plugins/shared/fleet/server/services/epm/kibana/assets/tag_assets.ts#L67
func isSharedTag(aId string, packageName string) bool {
	defaultSharedTagTemplate := "fleet-shared-tag-%s"
	securitySolutionTagTemplate := "%s-security-solution"

	return strings.Contains(aId, fmt.Sprintf(defaultSharedTagTemplate, packageName)) ||
		strings.Contains(aId, fmt.Sprintf(securitySolutionTagTemplate, packageName))
}

func filterOutSharedTags(ctx *transformationContext, references []interface{}) ([]interface{}, error) {
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

		aId, ok := reference["id"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to assert name as a string: %v", reference["id"])
		}
		if isSharedTag(aId, ctx.packageName) {
			continue
		}
		newReferences = append(newReferences, r)
	}
	return newReferences, nil
}
