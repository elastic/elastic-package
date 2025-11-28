// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"slices"
	"strings"

	"github.com/elastic/elastic-package/internal/packages"
)

// splitAndTrim splits a string by delimiter and trims whitespace from each element
func splitAndTrim(s, delimiter string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, delimiter)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// hasAnyMatch checks if any item in the items slice exists in the filters slice
func hasAnyMatch(filters []string, items []string) bool {
	if len(filters) == 0 {
		return true
	}

	for _, item := range items {
		if slices.Contains(filters, item) {
			return true
		}
	}

	return false
}

// extractInputs extracts all input types from package policy templates
func extractInputs(manifest *packages.PackageManifest) []string {
	uniqueInputs := make(map[string]struct{})
	for _, policyTemplate := range manifest.PolicyTemplates {
		if policyTemplate.Input != "" {
			uniqueInputs[policyTemplate.Input] = struct{}{}
		}

		for _, input := range policyTemplate.Inputs {
			uniqueInputs[input.Type] = struct{}{}
		}
	}

	inputs := make([]string, 0, len(uniqueInputs))
	for input := range uniqueInputs {
		inputs = append(inputs, input)
	}

	return inputs
}
