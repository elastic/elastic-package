// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/elastic/elastic-package/internal/logger"
)

type dynamicTemplate struct {
	name         string
	matchPattern string
	match        []string
	unmatch      []string
	pathMatch    []string
	unpathMatch  []string
	mapping      any
}

func (d *dynamicTemplate) Matches(currentPath string, definition map[string]any) (bool, error) {
	fullRegex := d.matchPattern == "regex"

	if len(d.match) > 0 {
		name := fieldNameFromPath(currentPath)
		if !slices.Contains(d.match, name) {
			// If there is no an exact match, it is compared with patterns/wildcards
			matches, err := stringMatchesPatterns(d.match, name, fullRegex)
			if err != nil {
				return false, fmt.Errorf("failed to parse dynamic template %q: %w", d.name, err)
			}

			if !matches {
				return false, nil
			}
		}
	}

	if len(d.unmatch) > 0 {
		name := fieldNameFromPath(currentPath)
		if slices.Contains(d.unmatch, name) {
			return false, nil
		}

		matches, err := stringMatchesPatterns(d.unmatch, name, fullRegex)
		if err != nil {
			return false, fmt.Errorf("failed to parse dynamic template %q: %w", d.name, err)
		}

		if matches {
			return false, nil
		}
	}

	if len(d.pathMatch) > 0 {
		matches, err := stringMatchesPatterns(d.pathMatch, currentPath, fullRegex)
		if err != nil {
			return false, fmt.Errorf("failed to parse dynamic template %s: %w", d.name, err)
		}
		if !matches {
			return false, nil
		}
	}

	if len(d.unpathMatch) > 0 {
		matches, err := stringMatchesPatterns(d.unpathMatch, currentPath, fullRegex)
		if err != nil {
			return false, fmt.Errorf("failed to parse dynamic template %q: %w", d.name, err)
		}
		if matches {
			return false, nil
		}
	}
	return true, nil
}

func stringMatchesRegex(regexes []string, elem string) (bool, error) {
	applies := false
	for _, v := range regexes {
		if !strings.Contains(v, "*") {
			// not a regex
			continue
		}

		match, err := regexp.MatchString(v, elem)
		if err != nil {
			return false, fmt.Errorf("failed to build regex %s: %w", v, err)
		}
		if match {
			applies = true
			break
		}
	}
	return applies, nil
}

func stringMatchesPatterns(regexes []string, elem string, fullRegex bool) (bool, error) {
	if fullRegex {
		return stringMatchesRegex(regexes, elem)
	}

	// transform wildcards to valid regexes
	updatedRegexes := []string{}
	for _, v := range regexes {
		r := strings.ReplaceAll(v, ".", "\\.")
		r = strings.ReplaceAll(r, "*", ".*")

		// Force to match the beginning and ending of the given path
		r = fmt.Sprintf("^%s$", r)

		updatedRegexes = append(updatedRegexes, r)
	}
	return stringMatchesRegex(updatedRegexes, elem)
}

func parseDynamicTemplates(rawDynamicTemplates []map[string]any) ([]dynamicTemplate, error) {
	dynamicTemplates := []dynamicTemplate{}

	for _, template := range rawDynamicTemplates {
		if len(template) != 1 {
			return nil, fmt.Errorf("unexpected number of dynamic template definitions found")
		}

		// there is just one dynamic template per object
		templateName := ""
		var rawContents any
		for key, value := range template {
			templateName = key
			rawContents = value
		}

		if shouldSkipDynamicTemplate(templateName) {
			continue
		}

		aDynamicTemplate := dynamicTemplate{
			name: templateName,
		}

		contents, ok := rawContents.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected dynamic template format found for %q", templateName)
		}

		isMatchAttributeDefined := false
		isRuntime := false
		for setting, value := range contents {
			switch setting {
			case "mapping":
				aDynamicTemplate.mapping = value
			case "runtime":
				isRuntime = true
			case "match_pattern":
				s, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("invalid type for \"match_pattern\": %T", value)
				}
				aDynamicTemplate.matchPattern = s
				isMatchAttributeDefined = true
			case "match":
				values, err := parseDynamicTemplateParameter(value)
				if err != nil {
					logger.Warnf("failed to check match setting: %s", err)
					return nil, fmt.Errorf("failed to check match setting: %w", err)
				}
				aDynamicTemplate.match = values
				isMatchAttributeDefined = true
			case "unmatch":
				values, err := parseDynamicTemplateParameter(value)
				if err != nil {
					return nil, fmt.Errorf("failed to check unmatch setting: %w", err)
				}
				aDynamicTemplate.unmatch = values
				isMatchAttributeDefined = true
			case "path_match":
				values, err := parseDynamicTemplateParameter(value)
				if err != nil {
					return nil, fmt.Errorf("failed to check path_match setting: %w", err)
				}
				aDynamicTemplate.pathMatch = values
				isMatchAttributeDefined = true
			case "path_unmatch":
				values, err := parseDynamicTemplateParameter(value)
				if err != nil {
					return nil, fmt.Errorf("failed to check path_unmatch setting: %w", err)
				}
				aDynamicTemplate.unpathMatch = values
				isMatchAttributeDefined = true
			case "match_mapping_type", "unmatch_mapping_type":
				isMatchAttributeDefined = true
				// Do nothing
				// These parameters require to check the original type (before the document is ingested)
				// but the dynamic template just contains the type from the `mapping` field
			default:
				return nil, fmt.Errorf("unexpected setting found in dynamic template")
			}
		}

		if isRuntime {
			continue
		}
		if !isMatchAttributeDefined {
			continue
		}
		dynamicTemplates = append(dynamicTemplates, aDynamicTemplate)
	}

	return dynamicTemplates, nil
}

func shouldSkipDynamicTemplate(templateName string) bool {
	// Filter out dynamic templates created by elastic-package (import_mappings)
	// or added automatically by ecs@mappings component template
	if strings.HasPrefix(templateName, "_embedded_ecs-") {
		return true
	}
	if strings.HasPrefix(templateName, "ecs_") {
		return true
	}
	if slices.Contains([]string{"all_strings_to_keywords", "strings_as_keyword"}, templateName) {
		return true
	}
	return false
}

func parseDynamicTemplateParameter(value any) ([]string, error) {
	all := []string{}
	switch v := value.(type) {
	case []any:
		for _, elem := range v {
			s, ok := elem.(string)
			if !ok {
				return nil, fmt.Errorf("failed to cast to string: %s", elem)
			}
			all = append(all, s)
		}
	case any:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("failed to cast to string: %s", v)
		}
		all = append(all, s)
	default:
		return nil, fmt.Errorf("unexpected type for setting: %T", value)

	}
	return all, nil
}
