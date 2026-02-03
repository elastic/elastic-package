// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tools

import (
	"embed"
	"fmt"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/parsing"
)

//go:embed _static/examples
var examplesFS embed.FS

// WildcardCategory is the special key for examples that are always returned
const WildcardCategory = "*"

// ExampleCategoryMap maps categories to example filenames.
// Use "*" as wildcard category for examples that should always be returned.
var ExampleCategoryMap = map[string][]string{
	"*": { // Always included
		"fortinet_fortigate.md",
	},
	"security": {
		"fortinet_fortigate.md",
	},
}

// ListExamplesArgs represents arguments for list_examples tool
type ListExamplesArgs struct {
	Categories []string `json:"categories"`
}

// ListExamplesResult represents the result of list_examples tool
type ListExamplesResult struct {
	Examples []string `json:"examples,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// GetExampleArgs represents arguments for get_example tool
type GetExampleArgs struct {
	Name    string `json:"name"`
	Section string `json:"section,omitempty"`
}

// GetExampleResult represents the result of get_example tool
type GetExampleResult struct {
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// listExamplesHandler returns a handler for the list_examples tool
func listExamplesHandler() functiontool.Func[ListExamplesArgs, ListExamplesResult] {
	return func(ctx tool.Context, args ListExamplesArgs) (ListExamplesResult, error) {
		seen := make(map[string]bool)
		var examples []string

		// Always include wildcard examples
		for _, example := range ExampleCategoryMap[WildcardCategory] {
			if !seen[example] {
				seen[example] = true
				examples = append(examples, example)
			}
		}

		// Add examples matching any of the requested categories (OR logic)
		for _, category := range args.Categories {
			categoryLower := strings.ToLower(strings.TrimSpace(category))
			for _, example := range ExampleCategoryMap[categoryLower] {
				if !seen[example] {
					seen[example] = true
					examples = append(examples, example)
				}
			}
		}

		if len(examples) == 0 {
			return ListExamplesResult{Error: "no examples found for the given categories"}, nil
		}

		return ListExamplesResult{Examples: examples}, nil
	}
}

// getExampleHandler returns a handler for the get_example tool
func getExampleHandler() functiontool.Func[GetExampleArgs, GetExampleResult] {
	return func(ctx tool.Context, args GetExampleArgs) (GetExampleResult, error) {
		if args.Name == "" {
			return GetExampleResult{Error: "example name is required"}, nil
		}

		// Read the example file from embedded FS
		// Note: embed.FS always uses forward slashes, regardless of OS
		filePath := "_static/examples/" + args.Name
		content, err := examplesFS.ReadFile(filePath)
		if err != nil {
			return GetExampleResult{Error: fmt.Sprintf("failed to read example file '%s': %v", args.Name, err)}, nil
		}

		// If no section specified, return the entire content
		if args.Section == "" {
			return GetExampleResult{Content: string(content)}, nil
		}

		// Parse sections and find the requested one using the parsing package
		sections := parsing.ParseSections(string(content))
		section := parsing.FindSectionByTitle(sections, args.Section)
		if section == nil {
			return GetExampleResult{Error: fmt.Sprintf("section '%s' not found in example '%s'", args.Section, args.Name)}, nil
		}

		return GetExampleResult{Content: section.FullContent}, nil
	}
}

// GetExampleContent retrieves the content of a specific example file.
// If section is provided, only that section's content is returned.
func GetExampleContent(name, section string) (string, error) {
	// Note: embed.FS always uses forward slashes, regardless of OS
	filePath := "_static/examples/" + name
	content, err := examplesFS.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read example file '%s': %w", name, err)
	}

	if section == "" {
		return string(content), nil
	}

	sections := parsing.ParseSections(string(content))
	sec := parsing.FindSectionByTitle(sections, section)
	if sec == nil {
		return "", fmt.Errorf("section '%s' not found in example '%s'", section, name)
	}

	return sec.FullContent, nil
}

// GetDefaultExampleContent returns the content of the first wildcard example.
// This is used for backward compatibility where a single example is needed.
func GetDefaultExampleContent() string {
	wildcardExamples := ExampleCategoryMap[WildcardCategory]
	if len(wildcardExamples) == 0 {
		return ""
	}

	content, err := GetExampleContent(wildcardExamples[0], "")
	if err != nil {
		return ""
	}
	return content
}

// CreateExampleTools creates the list_examples and get_example tools
func CreateExampleTools() []tool.Tool {
	var result []tool.Tool

	listExamplesTool, err := functiontool.New(
		functiontool.Config{
			Name:        "list_examples",
			Description: "List available example README files matching the integration categories. Examples demonstrate expected FORMAT and STYLE only - do not copy their content. Always returns wildcard examples plus any category-matched examples.",
		},
		listExamplesHandler(),
	)
	if err != nil {
		panic("failed to create list_examples tool: " + err.Error())
	}
	result = append(result, listExamplesTool)

	getExampleTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_example",
			Description: "Get content from a specific example README (optionally by section). Use for FORMAT and STYLE reference only - actual content must come from the package being documented, not the example.",
		},
		getExampleHandler(),
	)
	if err != nil {
		panic("failed to create get_example tool: " + err.Error())
	}
	result = append(result, getExampleTool)

	return result
}
