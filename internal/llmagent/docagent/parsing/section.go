// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package parsing provides markdown parsing utilities for documentation generation.
package parsing

import (
	"bufio"
	"strings"
)

// Section represents a parsed section from a markdown document
type Section struct {
	Title           string
	Level           int       // 2 for ##, 3 for ###
	Content         string    // Content ONLY for this section header, not subsections
	FullContent     string    // Full content including subsections (for generation)
	Subsections     []Section // Child sections
	StartLine       int
	EndLine         int
	HasPreserve     bool
	PreserveContent string
}

// IsTopLevel returns true if this is a level 2 section (has no parent)
func (s Section) IsTopLevel() bool {
	return s.Level == 2
}

// HasSubsections returns true if this section has children
func (s Section) HasSubsections() bool {
	return len(s.Subsections) > 0
}

// GetAllContent returns content including all subsections as markdown
func (s Section) GetAllContent() string {
	if s.FullContent != "" {
		return s.FullContent
	}
	return s.Content
}

// ContentLength returns the length of the section's full content
func (s Section) ContentLength() int {
	return len(s.GetAllContent())
}

// ParseSections extracts sections from markdown content based on headers (##, ###, ####, etc.)
// and builds a hierarchical tree where sections at level N+1 are children of sections at level N
func ParseSections(content string) []Section {
	var topLevelSections []*Section
	var sectionStack []*Section // Stack to track current path in the tree
	var currentSection *Section
	var contentBuffer strings.Builder

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check if this line is a header
		level, title := parseHeaderLine(line)

		if level > 0 {
			// Save accumulated content to current section
			if currentSection != nil {
				currentSection.Content = contentBuffer.String()
				currentSection.EndLine = lineNum - 1
			}

			// Create new section
			newSection := &Section{
				Title:       title,
				Level:       level,
				StartLine:   lineNum,
				Subsections: []Section{},
			}

			// Pop from stack until we find the appropriate parent
			// (parent must have level < current level)
			for len(sectionStack) > 0 && sectionStack[len(sectionStack)-1].Level >= level {
				sectionStack = sectionStack[:len(sectionStack)-1]
			}

			// Add new section to parent or top-level list
			if len(sectionStack) > 0 {
				// Add as subsection to parent
				parent := sectionStack[len(sectionStack)-1]
				parent.Subsections = append(parent.Subsections, *newSection)
				// Important: push the address of the slice element, not the original newSection
				// This ensures that future children are added to the correct Section in the slice
				sectionStack = append(sectionStack, &parent.Subsections[len(parent.Subsections)-1])
				currentSection = &parent.Subsections[len(parent.Subsections)-1]
			} else {
				// Add as top-level section
				topLevelSections = append(topLevelSections, newSection)
				// Push new section onto stack and make it current
				sectionStack = append(sectionStack, newSection)
				currentSection = newSection
			}

			// Start new content buffer with the header line
			contentBuffer.Reset()
			contentBuffer.WriteString(line)
			contentBuffer.WriteString("\n")

		} else {
			// Regular content line
			contentBuffer.WriteString(line)
			contentBuffer.WriteString("\n")

			// Check for PRESERVE blocks
			if currentSection != nil && strings.Contains(line, "<!-- PRESERVE START -->") {
				currentSection.HasPreserve = true
			}

			// Accumulate PRESERVE content
			if currentSection != nil && currentSection.HasPreserve {
				if !strings.Contains(currentSection.PreserveContent, "<!-- PRESERVE END -->") {
					currentSection.PreserveContent += line + "\n"
				}
			}
		}
	}

	// Finalize last section
	if currentSection != nil {
		currentSection.Content = contentBuffer.String()
		currentSection.EndLine = lineNum
	}

	// Convert pointers to values and build FullContent recursively
	var sections []Section
	for _, sec := range topLevelSections {
		sections = append(sections, buildSectionTree(sec))
	}

	return sections
}

// StartsWithHeader checks if a line starts with a markdown header of the given level and title
func StartsWithHeader(line, title string, level int) bool {
	lineLevel, lineTitle := parseHeaderLine(line)
	if lineLevel != level {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(lineTitle), strings.TrimSpace(title))
}

// parseHeaderLine checks if a line is a markdown header and returns its level and title
// Returns (0, "") if the line is not a header
func parseHeaderLine(line string) (level int, title string) {
	trimmed := strings.TrimLeft(line, " \t")

	// Count leading # characters
	hashCount := 0
	for i := 0; i < len(trimmed) && trimmed[i] == '#'; i++ {
		hashCount++
	}

	// Must have at least 2 # (we start at ##) and followed by a space
	if hashCount < 2 || hashCount >= len(trimmed) || trimmed[hashCount] != ' ' {
		return 0, ""
	}

	// Extract title (everything after "## ")
	title = strings.TrimSpace(trimmed[hashCount+1:])
	return hashCount, title
}

// buildSectionTree recursively builds the section tree, processing subsections
// and populating the FullContent field
func buildSectionTree(section *Section) Section {
	// Process subsections recursively
	var processedSubsections []Section
	for i := range section.Subsections {
		subsection := &section.Subsections[i]
		processedSubsections = append(processedSubsections, buildSectionTree(subsection))
	}
	section.Subsections = processedSubsections

	// Build FullContent: section's own content + all subsection contents
	BuildFullContent(section)

	return *section
}

// BuildFullContent populates FullContent field with section content plus all subsections.
// Exported for use by tests.
func BuildFullContent(section *Section) {
	// Extract own content (content before first subsection)
	ownContent := extractOwnContent(section.Content, section.Subsections)

	var builder strings.Builder
	builder.WriteString(ownContent)

	// Add all subsections
	for _, subsection := range section.Subsections {
		// Ensure proper spacing
		if builder.Len() > 0 {
			currentContent := builder.String()
			if !strings.HasSuffix(currentContent, "\n\n") {
				if strings.HasSuffix(currentContent, "\n") {
					builder.WriteString("\n")
				} else {
					builder.WriteString("\n\n")
				}
			}
		}
		builder.WriteString(subsection.GetAllContent())
	}

	section.FullContent = builder.String()

	// Update Content to only include own content (without subsections)
	section.Content = ownContent
}

// extractOwnContent extracts content that belongs only to this section
// (i.e., content before the first subsection header)
func extractOwnContent(content string, subsections []Section) string {
	if len(subsections) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	var ownLines []string

	// Find where the first subsection starts
	firstSubsectionLevel := subsections[0].Level
	headerPrefix := strings.Repeat("#", firstSubsectionLevel) + " "

	for _, line := range lines {
		if strings.HasPrefix(line, headerPrefix) {
			// Found first subsection - stop here
			break
		}
		ownLines = append(ownLines, line)
	}

	return strings.Join(ownLines, "\n")
}

// ParseSectionsToMap parses markdown content and returns a map of sections keyed by normalized (lowercase) title.
// This flattens the hierarchy into a single map for easy lookup.
func ParseSectionsToMap(content string) map[string]*Section {
	sections := ParseSections(content)
	flat := FlattenSections(sections)

	result := make(map[string]*Section)
	for i := range flat {
		normalizedTitle := strings.ToLower(strings.TrimSpace(flat[i].Title))
		// Only keep first occurrence if duplicate titles exist
		if _, exists := result[normalizedTitle]; !exists {
			result[normalizedTitle] = &flat[i]
		}
	}
	return result
}

// FindSectionByTitle finds a section with the given title (case-insensitive, fuzzy match)
func FindSectionByTitle(sections []Section, title string) *Section {
	titleLower := strings.ToLower(strings.TrimSpace(title))

	// First try exact match
	for i := range sections {
		if strings.ToLower(strings.TrimSpace(sections[i].Title)) == titleLower {
			return &sections[i]
		}
	}

	// Try fuzzy match (contains)
	for i := range sections {
		sectionTitleLower := strings.ToLower(strings.TrimSpace(sections[i].Title))
		if strings.Contains(sectionTitleLower, titleLower) || strings.Contains(titleLower, sectionTitleLower) {
			return &sections[i]
		}
	}

	return nil
}

// ExtractPreserveBlocks extracts all PRESERVE blocks from content
func ExtractPreserveBlocks(content string) []string {
	var blocks []string
	lines := strings.Split(content, "\n")
	var currentBlock []string
	inBlock := false

	for _, line := range lines {
		if strings.Contains(line, "<!-- PRESERVE START -->") {
			inBlock = true
			currentBlock = []string{line}
			continue
		}

		if inBlock {
			currentBlock = append(currentBlock, line)
			if strings.Contains(line, "<!-- PRESERVE END -->") {
				blocks = append(blocks, strings.Join(currentBlock, "\n"))
				currentBlock = []string{}
				inBlock = false
			}
		}
	}

	return blocks
}

// FlattenSections converts hierarchical sections to a flat list
// Useful for legacy code or specific use cases that need all sections as a flat list
func FlattenSections(sections []Section) []Section {
	var flat []Section

	for _, section := range sections {
		flat = append(flat, section)
		if len(section.Subsections) > 0 {
			flat = append(flat, section.Subsections...)
		}
	}

	return flat
}

// FindSectionByTitleHierarchical searches for a section in both top-level and subsections
// Returns the section if found at any level
func FindSectionByTitleHierarchical(sections []Section, title string) *Section {
	// Check top-level sections
	if section := FindSectionByTitle(sections, title); section != nil {
		return section
	}

	// Check subsections
	for i := range sections {
		if sub := FindSectionByTitle(sections[i].Subsections, title); sub != nil {
			return sub
		}
	}

	return nil
}

// GetParentSection finds the parent section that contains a given subsection title
// Returns nil if the section is top-level or not found
func GetParentSection(sections []Section, subsectionTitle string) *Section {
	for i := range sections {
		for _, subsection := range sections[i].Subsections {
			if strings.EqualFold(strings.TrimSpace(subsection.Title), strings.TrimSpace(subsectionTitle)) {
				return &sections[i]
			}
		}
	}
	return nil
}

// ExtractSectionByKeyword extracts content from a section by keyword(s).
// It searches for headers containing the keyword(s) and returns the section content
// up until the next same-level or higher-level heading.
// The keywords are matched case-insensitively.
func ExtractSectionByKeyword(content string, keywords []string) string {
	contentLower := strings.ToLower(content)

	for _, keyword := range keywords {
		keywordLower := strings.ToLower(keyword)

		// If keyword starts with #, treat it as a full header pattern
		var idx int
		var headerLevel int
		if strings.HasPrefix(keyword, "#") {
			idx = strings.Index(contentLower, keywordLower)
			if idx == -1 {
				continue
			}
			// Count # characters to determine level
			headerLevel = 0
			for _, c := range keyword {
				if c == '#' {
					headerLevel++
				} else {
					break
				}
			}
		} else {
			// Try "## keyword" first, then "### keyword"
			idx = strings.Index(contentLower, "## "+keywordLower)
			headerLevel = 2
			if idx == -1 {
				idx = strings.Index(contentLower, "### "+keywordLower)
				headerLevel = 3
			}
			if idx == -1 {
				continue
			}
		}

		// Find the start of the section content (after the header line)
		startIdx := idx
		newlineIdx := strings.Index(content[startIdx:], "\n")
		if newlineIdx != -1 {
			startIdx += newlineIdx + 1
		}

		// Find the next section at the same or higher level
		rest := content[startIdx:]
		lines := strings.Split(rest, "\n")
		var sectionContent []string

		for _, line := range lines {
			// Check if this is a header at the same or higher level
			if strings.HasPrefix(line, "#") {
				lineHeaderLevel := 0
				for _, c := range line {
					if c == '#' {
						lineHeaderLevel++
					} else {
						break
					}
				}
				if lineHeaderLevel > 0 && lineHeaderLevel <= headerLevel {
					// Found the next section at same or higher level
					break
				}
			}
			sectionContent = append(sectionContent, line)
		}

		result := strings.TrimSpace(strings.Join(sectionContent, "\n"))
		if result != "" {
			return result
		}
	}

	return ""
}
