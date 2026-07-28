// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/elastic/elastic-package/internal/common"
)

func cleanPolicyMap(policyMap common.MapStr, entries []policyEntryFilter) (common.MapStr, error) {
	for _, entry := range entries {
		v, err := policyMap.GetValue(entry.name)
		if errors.Is(err, common.ErrKeyNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}

		switch {
		case len(entry.elementsEntries) > 0:
			if err := applyElementsEntriesCleaning(policyMap, entry.name, v, entry.elementsEntries); err != nil {
				return nil, err
			}
		case len(entry.mapValues) > 0:
			if err := applyMapValuesCleaning(v, entry.mapValues); err != nil {
				return nil, err
			}
		case entry.memberReplace != nil:
			if err := applyMemberReplace(policyMap, entry.name, v, entry.memberReplace); err != nil {
				return nil, err
			}
		case entry.stringValueReplace != nil:
			if err := applyStringValueReplace(policyMap, entry.name, v, entry.stringValueReplace); err != nil {
				return nil, err
			}
		case entry.deletePattern != nil:
			if err := applyDeletePattern(policyMap, entry.name, v, entry.deletePattern); err != nil {
				return nil, err
			}
		default:
			if entry.onlyIfEmpty && !isEmpty(v, entry.ignoreValues) {
				continue
			}
			err := policyMap.Delete(entry.name)
			if errors.Is(err, common.ErrKeyNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
		}
	}

	return policyMap, nil
}

// replaceMapStrValue replaces the value stored at key in m by deleting it and putting the new value.
// Delete before Put is required because MapStr.Put does not overwrite slice/map values in place.
func replaceMapStrValue(m common.MapStr, key string, value any) error {
	m.Delete(key)
	_, err := m.Put(key, value)
	return err
}

// cleanPolicyMapStrSlice applies cleanPolicyMap to every element of list and returns a []any
// suitable for storing back into a MapStr.
func cleanPolicyMapStrSlice(list []common.MapStr, filters []policyEntryFilter) ([]any, error) {
	clean := make([]any, len(list))
	for i := range list {
		c, err := cleanPolicyMap(list[i], filters)
		if err != nil {
			return nil, err
		}
		clean[i] = c
	}
	return clean, nil
}

// cleanNestedPolicyValue applies filters to v, which must be either a single MapStr or a slice
// of MapStr values. It returns the cleaned value (MapStr or []any) to store back.
func cleanNestedPolicyValue(v any, filters []policyEntryFilter) (any, error) {
	if vMap, err := common.ToMapStr(v); err == nil {
		return cleanPolicyMap(vMap, filters)
	}
	if list, err := common.ToMapStrSlice(v); err == nil {
		return cleanPolicyMapStrSlice(list, filters)
	}
	return nil, fmt.Errorf("expected map or list, found %T", v)
}

// applyElementsEntriesCleaning applies filters to each element of the list stored at key.
func applyElementsEntriesCleaning(policyMap common.MapStr, key string, v any, filters []policyEntryFilter) error {
	list, err := common.ToMapStrSlice(v)
	if err != nil {
		return err
	}
	clean, err := cleanPolicyMapStrSlice(list, filters)
	if err != nil {
		return err
	}
	return replaceMapStrValue(policyMap, key, clean)
}

// applyMapValuesCleaning recurses into each value of the nested map stored in v,
// applying filters to every child (whether map or slice).
func applyMapValuesCleaning(v any, filters []policyEntryFilter) error {
	mapStr, err := common.ToMapStr(v)
	if err != nil {
		return err
	}
	for k, child := range mapStr {
		cleaned, err := cleanNestedPolicyValue(child, filters)
		if err != nil {
			return err
		}
		if err := replaceMapStrValue(mapStr, k, cleaned); err != nil {
			return err
		}
	}
	return nil
}

// applyMemberReplace applies the regexp replacement to keys (for MapStr) or string elements (for []any).
func applyMemberReplace(policyMap common.MapStr, key string, v any, r *policyEntryReplace) error {
	switch val := v.(type) {
	case common.MapStr:
		for k, e := range val {
			if r.regexp.MatchString(k) {
				delete(val, k)
				val[r.regexp.ReplaceAllString(k, r.replace)] = e
			}
		}
		return nil
	case []any:
		replaced, err := mapStringElemsInAnySlice(val, func(s string) string {
			if r.regexp.MatchString(s) {
				return r.regexp.ReplaceAllString(s, r.replace)
			}
			return s
		})
		if err != nil {
			return err
		}
		return replaceMapStrValue(policyMap, key, replaced)
	default:
		return fmt.Errorf("expected map or array for memberReplace, found %T", v)
	}
}

// applyStringValueReplace applies the regexp replacement to a single string value at key.
func applyStringValueReplace(policyMap common.MapStr, key string, v any, r *policyEntryReplace) error {
	vStr, ok := v.(string)
	if !ok {
		return fmt.Errorf("expected string, found %T", v)
	}
	if r.regexp.MatchString(vStr) {
		policyMap.Put(key, r.regexp.ReplaceAllString(vStr, r.replace)) //nolint:errcheck // single string Put cannot fail
	}
	return nil
}

// applyDeletePattern removes matching keys (for MapStr) or matching string elements (for []any).
func applyDeletePattern(policyMap common.MapStr, key string, v any, pattern *regexp.Regexp) error {
	switch val := v.(type) {
	case common.MapStr:
		for k := range val {
			if pattern.MatchString(k) {
				delete(val, k)
			}
		}
		return nil
	case []any:
		filtered, err := filterStringElemsInAnySlice(val, func(s string) bool {
			return !pattern.MatchString(s)
		})
		if err != nil {
			return err
		}
		return replaceMapStrValue(policyMap, key, filtered)
	default:
		return fmt.Errorf("expected map or array for deletePattern, found %T", v)
	}
}

// mapStringElemsInAnySlice applies f to every string element of val.
// Returns an error if any element is not a string.
func mapStringElemsInAnySlice(val []any, f func(string) string) ([]any, error) {
	out := make([]any, len(val))
	for i, elem := range val {
		s, ok := elem.(string)
		if !ok {
			return nil, fmt.Errorf("expected string array element, found %T", elem)
		}
		out[i] = f(s)
	}
	return out, nil
}

// filterStringElemsInAnySlice returns a new slice containing only elements for which keep returns true.
// Returns an error if any element is not a string.
func filterStringElemsInAnySlice(val []any, keep func(string) bool) ([]any, error) {
	filtered := val[:0]
	for _, elem := range val {
		s, ok := elem.(string)
		if !ok {
			return nil, fmt.Errorf("expected string array element, found %T", elem)
		}
		if keep(s) {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

// isEmpty checks if the value is empty. It is considered empty if it is the zero value,
// or for values for length, if it is zero. Values in ignoreValues are not counted for
// the total length when present in lists.
func isEmpty(v any, ignoreValues []any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case []any:
		return len(filterIgnored(v, ignoreValues)) == 0
	case map[string]any:
		return len(v) == 0
	case common.MapStr:
		return len(v) == 0
	}

	return false
}

func filterIgnored(v []any, ignoredValues []any) []any {
	return slices.DeleteFunc(v, func(e any) bool {
		return slices.Contains(ignoredValues, e)
	})
}
