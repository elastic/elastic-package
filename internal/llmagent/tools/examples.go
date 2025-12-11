// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tools

import (
	"bufio"
	"embed"
	"fmt"
	"path"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

//go:embed _static/examples/*.md
var examplesFS embed.FS

// WildcardCategory is the special key for examples that are always returned
const WildcardCategory = "*"

// ExampleCategoryMap maps categories to example filenames.
// Use "*" as wildcard category for examples that should always be returned.
var ExampleCategoryMap = map[string][]string{
	"*": { // Always included
	},
	"security": {
		"fortinet_fortigate.md",
	},
	"observability": {
		"proofpoint_essentials.md",
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
		filePath := path.Join("_static/examples", args.Name)
		content, err := examplesFS.ReadFile(filePath)
		if err != nil {
			return GetExampleResult{Error: fmt.Sprintf("failed to read example file '%s': %v", args.Name, err)}, nil
		}

		// If no section specified, return the entire content
		if args.Section == "" {
			return GetExampleResult{Content: string(content)}, nil
		}

		// Parse sections and find the requested one
		sections := parseExampleSections(string(content))
		section := findSectionByTitle(sections, args.Section)
		if section == nil {
			return GetExampleResult{Error: fmt.Sprintf("section '%s' not found in example '%s'", args.Section, args.Name)}, nil
		}

		return GetExampleResult{Content: section.FullContent}, nil
	}
}

// exampleSection represents a parsed section from a markdown document
type exampleSection struct {
	Title       string
	Level       int
	Content     string
	FullContent string
	Subsections []*exampleSection
}

// parseExampleSections extracts sections from markdown content based on headers
func parseExampleSections(content string) []*exampleSection {
	var topLevelSections []*exampleSection
	var sectionStack []*exampleSection
	var currentSection *exampleSection
	var contentBuffer strings.Builder

	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := scanner.Text()
		level, title := parseExampleHeaderLine(line)

		if level > 0 {
			// Save accumulated content to current section
			if currentSection != nil {
				currentSection.Content = contentBuffer.String()
			}

			// Create new section
			newSection := &exampleSection{
				Title:       title,
				Level:       level,
				Subsections: []*exampleSection{},
			}

			// Pop from stack until we find appropriate parent
			for len(sectionStack) > 0 && sectionStack[len(sectionStack)-1].Level >= level {
				sectionStack = sectionStack[:len(sectionStack)-1]
			}

			// Add new section to parent or top-level list
			if len(sectionStack) > 0 {
				parent := sectionStack[len(sectionStack)-1]
				parent.Subsections = append(parent.Subsections, newSection)
			} else {
				topLevelSections = append(topLevelSections, newSection)
			}

			sectionStack = append(sectionStack, newSection)
			currentSection = newSection

			// Start new content buffer with the header line
			contentBuffer.Reset()
			contentBuffer.WriteString(line)
			contentBuffer.WriteString("\n")
		} else {
			contentBuffer.WriteString(line)
			contentBuffer.WriteString("\n")
		}
	}

	// Finalize last section
	if currentSection != nil {
		currentSection.Content = contentBuffer.String()
	}

	// Build FullContent for each section
	for _, sec := range topLevelSections {
		buildExampleFullContent(sec)
	}

	return topLevelSections
}

// parseExampleHeaderLine checks if a line is a markdown header
func parseExampleHeaderLine(line string) (level int, title string) {
	trimmed := strings.TrimLeft(line, " \t")

	hashCount := 0
	for i := 0; i < len(trimmed) && trimmed[i] == '#'; i++ {
		hashCount++
	}

	if hashCount < 2 || hashCount >= len(trimmed) || trimmed[hashCount] != ' ' {
		return 0, ""
	}

	title = strings.TrimSpace(trimmed[hashCount+1:])
	return hashCount, title
}

// buildExampleFullContent populates FullContent field with section content plus all subsections
func buildExampleFullContent(section *exampleSection) {
	var builder strings.Builder
	builder.WriteString(section.Content)

	for _, subsection := range section.Subsections {
		buildExampleFullContent(subsection)
		builder.WriteString(subsection.FullContent)
	}

	section.FullContent = builder.String()
}

// findSectionByTitle finds a section with the given title (case-insensitive)
func findSectionByTitle(sections []*exampleSection, title string) *exampleSection {
	titleLower := strings.ToLower(strings.TrimSpace(title))

	// Check top-level sections first
	for _, sec := range sections {
		if strings.ToLower(strings.TrimSpace(sec.Title)) == titleLower {
			return sec
		}
	}

	// Check subsections
	for _, sec := range sections {
		if found := findSectionByTitleInSubsections(sec.Subsections, titleLower); found != nil {
			return found
		}
	}

	// Try fuzzy match (contains)
	for _, sec := range sections {
		secTitleLower := strings.ToLower(strings.TrimSpace(sec.Title))
		if strings.Contains(secTitleLower, titleLower) || strings.Contains(titleLower, secTitleLower) {
			return sec
		}
	}

	return nil
}

// findSectionByTitleInSubsections recursively searches subsections
func findSectionByTitleInSubsections(sections []*exampleSection, titleLower string) *exampleSection {
	for _, sec := range sections {
		if strings.ToLower(strings.TrimSpace(sec.Title)) == titleLower {
			return sec
		}
		if found := findSectionByTitleInSubsections(sec.Subsections, titleLower); found != nil {
			return found
		}
	}
	return nil
}

// GetExampleContent retrieves the content of a specific example file.
// If section is provided, only that section's content is returned.
func GetExampleContent(name, section string) (string, error) {
	filePath := path.Join("_static/examples", name)
	content, err := examplesFS.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read example file '%s': %w", name, err)
	}

	if section == "" {
		return string(content), nil
	}

	sections := parseExampleSections(string(content))
	sec := findSectionByTitle(sections, section)
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
