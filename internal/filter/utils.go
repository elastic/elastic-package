package filter

import (
	"strings"

	"github.com/elastic/elastic-package/internal/packages"
)

// splitAndTrim splits a string by delimiter and trims whitespace from each element
func splitAndTrim(s, delimiter string) map[string]struct{} {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, delimiter)
	result := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

// hasAnyMatch checks if any item in the items slice exists in the filters slice
func hasAnyMatch(filters map[string]struct{}, items []string) bool {
	if len(filters) == 0 {
		return true
	}

	for _, item := range items {
		if _, ok := filters[item]; ok {
			return true
		}
	}

	return false
}

// extractInputs extracts all input types from package policy templates
func extractInputs(pkg packages.PackageManifest) []string {
	var inputs []string
	uniqueInputs := make(map[string]struct{})
	for _, policyTemplate := range pkg.PolicyTemplates {
		if policyTemplate.Input != "" {
			uniqueInputs[policyTemplate.Input] = struct{}{}
		}

		for _, input := range policyTemplate.Inputs {
			uniqueInputs[input.Type] = struct{}{}
		}
	}

	for input := range uniqueInputs {
		inputs = append(inputs, input)
	}

	return inputs
}
