// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"

	"github.com/elastic/go-resource"
)

const (
	testRunMaxID = 99999
	testRunMinID = 10000
)

// TrimStringSlice removes whitespace from the beginning and end of the contents of a []string.
func TrimStringSlice(slice []string) {
	for iterator, item := range slice {
		slice[iterator] = strings.TrimSpace(item)
	}
}

// StringSlicesUnion joins multiple slices and returns an slice with the distinct
// elements of all of them.
func StringSlicesUnion(ss ...[]string) (result []string) {
	for _, slice := range ss {
		for _, elem := range slice {
			if slices.Contains(result, elem) {
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

func CreateTestRunID() string {
	return CreateTestRunIDWithPrefix("")
}

func CreateTestRunIDWithPrefix(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, rand.Intn(testRunMaxID-testRunMinID)+testRunMinID)
}

func ProcessResourceApplyResults(results resource.ApplyResults) string {
	var errors []string
	for _, result := range results {
		if err := result.Err(); err != nil {
			errors = append(errors, err.Error())
		}
	}
	return strings.Join(errors, ", ")
}
