package filter

import "strings"

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
