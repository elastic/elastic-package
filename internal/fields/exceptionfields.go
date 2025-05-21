// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"slices"
	"strings"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
)

// ListExceptionFields validates the provided document as common.MapStr.
func (v *Validator) ListExceptionFields(body common.MapStr) []string {
	return v.listExceptionFieldsMapElement("", body)
}

func (v *Validator) listExceptionFieldsMapElement(root string, elem common.MapStr) []string {
	all := []string{}
	for name, val := range elem {
		key := strings.TrimLeft(root+"."+name, ".")

		switch val := val.(type) {
		case []map[string]any:
			for _, m := range val {
				fields := v.listExceptionFieldsMapElement(key, m)
				all = append(all, fields...)
			}
		case map[string]any:
			if isFieldTypeFlattened(key, v.Schema) {
				// Do not traverse into objects with flattened data types
				// because the entire object is mapped as a single field.
				continue
			}
			fields := v.listExceptionFieldsMapElement(key, val)
			all = append(all, fields...)
		default:
			if skipLeafOfObject(root, name, v.specVersion, v.Schema) {
				logger.Tracef("Skip validating leaf of object (spec %q): %q", v.specVersion, key)
				all = append(all, key)
				// Till some versions we skip some validations on leaf of objects, check if it is the case.
				break
			}

			fields := v.listExceptionFieldsScalarElement(key, val)
			all = append(all, fields...)
		}
	}
	slices.Sort(all)
	all = slices.Compact(all)
	return all
}

func (v *Validator) listExceptionFieldsScalarElement(key string, val any) []string {
	if key == "" {
		return nil // root key is always valid
	}

	definition := FindElementDefinition(key, v.Schema)
	if definition == nil {
		return nil
	}

	return findElements(key, *definition, val, v.parseExceptionField)
}

// findElements visits a function for each element in the given value if
// it is an array. If it is not an array, it calls the function with it.
func findElements(key string, definition FieldDefinition, val any, fn func(string, FieldDefinition, any) []string) []string {
	arr, isArray := val.([]any)
	if !isArray {
		return fn(key, definition, val)
	}
	var all []string
	for _, element := range arr {
		fields := fn(key, definition, element)
		all = append(all, fields...)
	}
	return all
}

// parseExceptionField performs validations on individual values of each element.
func (v *Validator) parseExceptionField(key string, definition FieldDefinition, val any) []string {
	switch definition.Type {
	case "constant_keyword":
	case "keyword", "text":
	case "date":
	case "float", "long", "double":
	case "ip":
	case "array":
		if v.specVersion.LessThan(semver2_0_0) {
			logger.Tracef("Skip validating field of type array with spec < 2.0.0 (key %q type %q spec %q)", key, definition.Type, v.specVersion)
			return []string{key}
		}
		return nil
	// Groups should only contain nested fields, not single values.
	case "group", "nested", "object":
		switch val := val.(type) {
		case map[string]any:
			// This is probably an element from an array of objects,
			// even if not recommended, it should be validated.
			if v.specVersion.LessThan(semver3_0_1) {
				logger.Tracef("Skip validating object (map[string]any) in package spec < 3.0.1 (key %q type %q spec %q)", key, definition.Type, v.specVersion)
				return []string{key}
			}
			return nil
		case []any:
			// This can be an array of array of objects. Elasticsearh will probably
			// flatten this. So even if this is quite unexpected, let's try to handle it.
			if v.specVersion.LessThan(semver3_0_1) {
				logger.Tracef("Skip validating object ([]any) because spec < 3.0.1 (key %q type %q spec %q)", key, definition.Type, v.specVersion)
				return []string{key}
			}
			return nil
		case nil:
			// The document contains a null, let's consider this like an empty array.
			logger.Tracef("Skip validating object empty array because spec < 3.0.1 (key %q type %q spec %q)", key, definition.Type, v.specVersion)
			return []string{key}
		default:
			switch {
			case definition.Type == "object" && definition.ObjectType != "":
				// This is the leaf element of an object without wildcards in the name, adapt the definition and try again.
				definition.Name = definition.Name + ".*"
				definition.Type = definition.ObjectType
				definition.ObjectType = ""
				return v.parseExceptionField(key, definition, val)
			case definition.Type == "object" && definition.ObjectType == "":
				// Legacy mapping, ambiguous definition not allowed by recent versions of the spec, ignore it.
				logger.Tracef("Skip legacy mapping: object field without \"object_type\" parameter: %q", key)
				return []string{key}
			}

			return nil
		}
	default:
		return nil
	}

	return nil
}
