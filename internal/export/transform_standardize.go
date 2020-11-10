// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
)

const dashboardLinkPrefix = "#/dashboard/"

func standardizeObjectID(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	// Adjust object ID
	id, _ := object.GetValue("id")
	_, err := object.Put("id", adjustObjectID(ctx, id.(string)))
	if err != nil {
		return nil, errors.Wrapf(err, "can't update object ID")
	}

	// Adjust references
	references, err := object.GetValue("references")
	if err != nil && err != common.ErrKeyNotFound {
		return nil, errors.Wrap(err, "retrieving object references failed")
	}

	newReferences, err := adjustObjectReferences(ctx, references.([]interface{}))
	if err != nil {
		return nil, errors.Wrapf(err, "can't adjust object references (ID: %s)", id)
	}

	_, err = object.Put("references", newReferences)
	if err != nil {
		return nil, errors.Wrapf(err, "can't update references")
	}
	return object, nil
}

func adjustObjectReferences(ctx *transformationContext, references []interface{}) ([]interface{}, error) {
	for i, r := range references {
		reference := r.(map[string]interface{})
		if aType, ok := reference["type"]; ok && aType == "index-pattern" {
			continue // don't modify ID for index-patterns
		}

		if id, ok := reference["id"]; ok {
			newID := adjustObjectID(ctx, id.(string))
			reference["id"] = newID
			references[i] = reference
		}
	}
	return references, nil
}

func standardizeObjectProperties(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	for key, value := range object {
		if key == "title" {
			_, err := object.Put(key, adjustTitleProperty(value.(string)))
			if err != nil {
				return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
			}
			continue
		}

		if key == "markdown" {
			_, err := object.Put(key, adjustMarkdownProperty(ctx, value.(string)))
			if err != nil {
				return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
			}
			continue
		}

		var target interface{}
		var err error
		var updated bool

		switch value.(type) {
		case map[string]interface{}:
			target, err = standardizeObjectProperties(ctx, value.(map[string]interface{}))
			if err != nil {
				return nil, errors.Wrapf(err, "can't standardize object (key: %s)", key)
			}
			updated = true
		case []map[string]interface{}:
			arr := value.([]map[string]interface{})
			for i, obj := range arr {
				newValue, err := standardizeObjectProperties(ctx, obj)
				if err != nil {
					return nil, errors.Wrapf(err, "can't standardize object (array index: %d)", i)
				}
				arr[i] = newValue
			}
			target = arr
			updated = true
		}

		if !updated {
			continue
		}

		_, err = object.Put(key, target)
		if err != nil {
			return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
		}
	}
	return object, nil
}

func adjustTitleProperty(title string) string {
	if strings.HasSuffix(title, " ECS") {
		return strings.ReplaceAll(title, " ECS", "")
	}
	return title
}

func adjustMarkdownProperty(ctx *transformationContext, content string) string {
	r := regexp.MustCompile("(" + dashboardLinkPrefix + "[^)]+)")
	return r.ReplaceAllStringFunc(content, func(match string) string {
		match = match[len(dashboardLinkPrefix):]
		return dashboardLinkPrefix + adjustObjectID(ctx, match)
	})
}

func adjustObjectID(ctx *transformationContext, id string) string {
	// If object ID starts with the package name, make sure that package name is all lowercase
	// Else, prefix an all-lowercase module name to the object ID.
	newID := id
	prefix := ctx.packageName + "-"
	if strings.HasPrefix(strings.ToLower(newID), prefix) {
		newID = newID[len(prefix):]
	}
	newID = prefix + newID

	// If object ID ends with "-ecs", trim it off.
	if strings.HasSuffix(newID, "-ecs") {
		newID = strings.TrimSuffix(newID, "-ecs")
	}
	return newID
}
