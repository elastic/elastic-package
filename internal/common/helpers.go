// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"fmt"
	"strings"
)

// TrimStringSlice removes whitespace from the beginning and end of the contents of a []string.
func TrimStringSlice(slice []string) {
	for iterator, item := range slice {
		slice[iterator] = strings.TrimSpace(item)
	}
}

// StringSliceContains checks if the slice contains the given string.
func StringSliceContains(slice []string, s string) bool {
	for i := range slice {
		if slice[i] == s {
			return true
		}
	}
	return false
}

// StringSlicesUnion joins multiple slices and returns an slice with the distinct
// elements of all of them.
func StringSlicesUnion(slices ...[]string) (result []string) {
	for _, slice := range slices {
		for _, elem := range slice {
			if StringSliceContains(result, elem) {
				continue
			}
			result = append(result, elem)
		}
	}
	return
}

// ToStringSlice returns the list of strings from an interface variable
func ToStringSlice(val interface{}) ([]string, error) {
	vals, ok := val.([]interface{})
	if !ok {
		return nil, fmt.Errorf("conversion error")
	}

	var s []string
	for _, v := range vals {
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("conversion error")
		}
		s = append(s, str)
	}
	return s, nil
}
