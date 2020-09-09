// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

// WARNING: This code is copied from https://github.com/elastic/beats/blob/master/libbeat/common/mapstr.go
// This was done to not have to import the full common package and all its dependencies
// Not needed methods / variables were removed, but no changes made to the logic.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

var (
	// errKeyNotFound indicates that the specified key was not found.
	errKeyNotFound = errors.New("key not found")
)

// MapStr is a map[string]interface{} wrapper with utility methods for common
// map operations like converting to JSON.
type MapStr map[string]interface{}

// GetValue gets a value from the map. If the key does not exist then an error
// is returned.
func (m MapStr) GetValue(key string) (interface{}, error) {
	_, _, v, found, err := mapFind(key, m, false)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errKeyNotFound
	}
	return v, nil
}

// Put associates the specified value with the specified key. If the map
// previously contained a mapping for the key, the old value is replaced and
// returned. The key can be expressed in dot-notation (e.g. x.y) to put a value
// into a nested map.
//
// If you need insert keys containing dots then you must use bracket notation
// to insert values (e.g. m[key] = value).
func (m MapStr) Put(key string, value interface{}) (interface{}, error) {
	// XXX `safemapstr.Put` mimics this implementation, both should be updated to have similar behavior
	k, d, old, _, err := mapFind(key, m, true)
	if err != nil {
		return nil, err
	}

	d[k] = value
	return old, nil
}

// StringToPrint returns the MapStr as pretty JSON.
func (m MapStr) StringToPrint() string {
	j, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Sprintf("Not valid json: %v", err)
	}
	return string(j)
}

// tomapStr performs a type assertion on v and returns a MapStr. v can be either
// a MapStr or a map[string]interface{}. If it's any other type or nil then
// an error is returned.
func toMapStr(v interface{}) (MapStr, error) {
	m, ok := tryToMapStr(v)
	if !ok {
		return nil, errors.Errorf("expected map but type is %T", v)
	}
	return m, nil
}

func tryToMapStr(v interface{}) (MapStr, bool) {
	switch m := v.(type) {
	case MapStr:
		return m, true
	case map[string]interface{}:
		return MapStr(m), true
	default:
		return nil, false
	}
}

// mapFind iterates a MapStr based on a the given dotted key, finding the final
// subMap and subKey to operate on.
// An error is returned if some intermediate is no map or the key doesn't exist.
// If createMissing is set to true, intermediate maps are created.
// The final map and un-dotted key to run further operations on are returned in
// subKey and subMap. The subMap already contains a value for subKey, the
// present flag is set to true and the oldValue return will hold
// the original value.
func mapFind(
	key string,
	data MapStr,
	createMissing bool,
) (subKey string, subMap MapStr, oldValue interface{}, present bool, err error) {
	// XXX `safemapstr.mapFind` mimics this implementation, both should be updated to have similar behavior

	for {
		// Fast path, key is present as is.
		if v, exists := data[key]; exists {
			return key, data, v, true, nil
		}

		idx := strings.IndexRune(key, '.')
		if idx < 0 {
			return key, data, nil, false, nil
		}

		k := key[:idx]
		d, exists := data[k]
		if !exists {
			if createMissing {
				d = MapStr{}
				data[k] = d
			} else {
				return "", nil, nil, false, errKeyNotFound
			}
		}

		v, err := toMapStr(d)
		if err != nil {
			return "", nil, nil, false, err
		}

		// advance to sub-map
		key = key[idx+1:]
		data = v
	}
}
