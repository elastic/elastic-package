// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

// WARNING: This code is copied from https://github.com/elastic/beats/blob/master/libbeat/common/mapstr.go
// This was done to not have to import the full common package and all its dependencies
// Not needed methods / variables were removed, but no changes made to the logic.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrKeyNotFound indicates that the specified key was not found.
	ErrKeyNotFound = errors.New("key not found")
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
		return nil, ErrKeyNotFound
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

// DeepUpdate recursively copies the key-value pairs from d to this map.
// If the key is present and a map as well, the sub-map will be updated recursively
// via DeepUpdate.
// DeepUpdateNoOverwrite is a version of this function that does not
// overwrite existing values.
func (m MapStr) DeepUpdate(d MapStr) {
	m.deepUpdateMap(d, true)
}

// DeepUpdateNoOverwrite recursively copies the key-value pairs from d to this map.
// If a key is already present it will not be overwritten.
// DeepUpdate is a version of this function that overwrites existing values.
func (m MapStr) DeepUpdateNoOverwrite(d MapStr) {
	m.deepUpdateMap(d, false)
}

func (m MapStr) deepUpdateMap(d MapStr, overwrite bool) {
	for k, v := range d {
		switch val := v.(type) {
		case map[string]interface{}:
			m[k] = deepUpdateValue(m[k], MapStr(val), overwrite)
		case MapStr:
			m[k] = deepUpdateValue(m[k], val, overwrite)
		default:
			if overwrite {
				m[k] = v
			} else if _, exists := m[k]; !exists {
				m[k] = v
			}
		}
	}
}

func deepUpdateValue(old interface{}, val MapStr, overwrite bool) interface{} {
	switch sub := old.(type) {
	case MapStr:
		if sub == nil {
			return val
		}

		sub.deepUpdateMap(val, overwrite)
		return sub
	case map[string]interface{}:
		if sub == nil {
			return val
		}

		tmp := MapStr(sub)
		tmp.deepUpdateMap(val, overwrite)
		return tmp
	default:
		// We reach the default branch if old is no map or if old == nil.
		// In either case we return `val`, such that the old value is completely
		// replaced when merging.
		return val
	}
}

// Delete deletes the given key from the map.
func (m MapStr) Delete(key string) error {
	k, d, _, found, err := mapFind(key, m, false)
	if err != nil {
		return err
	}
	if !found {
		return ErrKeyNotFound
	}

	delete(d, k)
	return nil
}

// StringToPrint returns the MapStr as pretty JSON.
func (m MapStr) StringToPrint() string {
	j, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Sprintf("Not valid json: %v", err)
	}
	return string(j)
}

// ToMapStrSlice function tries to convert the interface into the slice of MapStrs.
func ToMapStrSlice(slice interface{}) ([]MapStr, error) {
	sliceI, ok := slice.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected slice of interfaces but type is %T", slice)
	}

	var mapStrs []MapStr
	for _, v := range sliceI {
		m, err := ToMapStr(v)
		if err != nil {
			return nil, fmt.Errorf("can't convert element to MapStr: %w", err)
		}
		mapStrs = append(mapStrs, m)
	}
	return mapStrs, nil
}

// ToMapStr performs a type assertion on v and returns a MapStr. v can be either
// a MapStr or a map[string]interface{}. If it's any other type or nil then
// an error is returned.
func ToMapStr(v interface{}) (MapStr, error) {
	m, ok := tryToMapStr(v)
	if !ok {
		return nil, fmt.Errorf("expected map but type is %T", v)
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
				return "", nil, nil, false, ErrKeyNotFound
			}
		}

		v, err := ToMapStr(d)
		if err != nil {
			return "", nil, nil, false, err
		}

		// advance to sub-map
		key = key[idx+1:]
		data = v
	}
}
