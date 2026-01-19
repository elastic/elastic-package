// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/parsing"
)

func TestParseSections_Hierarchical(t *testing.T) {
	tests := []struct {
		name                string
		content             string
		expectedSections    int
		expectedSubsections map[string]int // section title -> subsection count
	}{
		{
			name: "simple hierarchy with subsections",
			content: `## Overview

This is overview content.

### Compatibility

Compatibility info here.

### How it works

How it works info.

## What data

Data collection info.`,
			expectedSections: 2,
			expectedSubsections: map[string]int{
				"Overview":  2,
				"What data": 0,
			},
		},
		{
			name: "flat structure (no subsections)",
			content: `## Section 1

Content 1.

## Section 2

Content 2.`,
			expectedSections: 2,
			expectedSubsections: map[string]int{
				"Section 1": 0,
				"Section 2": 0,
			},
		},
		{
			name: "multiple subsections",
			content: `## Deployment

Deployment intro.

### Prerequisites

Prereq info.

### Configuration

Config info.

### Verification

Verify info.

## Troubleshooting

Troubleshooting content.`,
			expectedSections: 2,
			expectedSubsections: map[string]int{
				"Deployment":      3,
				"Troubleshooting": 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections := parsing.ParseSections(tt.content)

			// Check number of top-level sections
			assert.Len(t, sections, tt.expectedSections, "number of top-level sections")

			// Check subsection counts
			for sectionTitle, expectedSubCount := range tt.expectedSubsections {
				found := false
				for _, section := range sections {
					if section.Title == sectionTitle {
						found = true
						assert.Len(t, section.Subsections, expectedSubCount,
							"subsection count for %s", sectionTitle)
						break
					}
				}
				assert.True(t, found, "section %s not found", sectionTitle)
			}
		})
	}
}

func TestSection_IsTopLevel(t *testing.T) {
	level2 := Section{Level: 2}
	level3 := Section{Level: 3}

	assert.True(t, level2.IsTopLevel())
	assert.False(t, level3.IsTopLevel())
}

func TestSection_HasSubsections(t *testing.T) {
	withSubs := Section{
		Subsections: []Section{{Title: "Sub1"}},
	}
	withoutSubs := Section{
		Subsections: []Section{},
	}

	assert.True(t, withSubs.HasSubsections())
	assert.False(t, withoutSubs.HasSubsections())
}

func TestSection_GetAllContent(t *testing.T) {
	section := Section{
		Content:     "Main content",
		FullContent: "Full content with subs",
	}

	assert.Equal(t, "Full content with subs", section.GetAllContent())

	// Test fallback when FullContent is empty
	section.FullContent = ""
	assert.Equal(t, "Main content", section.GetAllContent())
}

func TestBuildFullContent(t *testing.T) {
	section := Section{
		Title:   "Parent",
		Level:   2,
		Content: "## Parent\n\nParent content.",
		Subsections: []Section{
			{
				Title:   "Child1",
				Level:   3,
				Content: "### Child1\n\nChild1 content.",
			},
			{
				Title:   "Child2",
				Level:   3,
				Content: "### Child2\n\nChild2 content.",
			},
		},
	}

	parsing.BuildFullContent(&section)

	assert.NotEmpty(t, section.FullContent)
	assert.Contains(t, section.FullContent, "Parent content")
	assert.Contains(t, section.FullContent, "Child1 content")
	assert.Contains(t, section.FullContent, "Child2 content")
}

func TestFlattenSections(t *testing.T) {
	hierarchical := []Section{
		{
			Title: "Parent1",
			Level: 2,
			Subsections: []Section{
				{Title: "Child1", Level: 3},
				{Title: "Child2", Level: 3},
			},
		},
		{
			Title:       "Parent2",
			Level:       2,
			Subsections: []Section{},
		},
	}

	flat := parsing.FlattenSections(hierarchical)

	// Should have 4 total: Parent1, Child1, Child2, Parent2
	assert.Len(t, flat, 4)
	assert.Equal(t, "Parent1", flat[0].Title)
	assert.Equal(t, "Child1", flat[1].Title)
	assert.Equal(t, "Child2", flat[2].Title)
	assert.Equal(t, "Parent2", flat[3].Title)
}

func TestFindSectionByTitleHierarchical(t *testing.T) {
	sections := []Section{
		{
			Title: "Overview",
			Level: 2,
			Subsections: []Section{
				{Title: "Compatibility", Level: 3},
				{Title: "How it works", Level: 3},
			},
		},
		{
			Title: "Deployment",
			Level: 2,
		},
	}

	tests := []struct {
		name          string
		searchTitle   string
		shouldFind    bool
		expectedLevel int
	}{
		{"find top-level", "Overview", true, 2},
		{"find top-level 2", "Deployment", true, 2},
		{"find subsection", "Compatibility", true, 3},
		{"find subsection 2", "How it works", true, 3},
		{"not found", "NonExistent", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsing.FindSectionByTitleHierarchical(sections, tt.searchTitle)
			if tt.shouldFind {
				require.NotNil(t, result, "should find section")
				assert.Equal(t, tt.expectedLevel, result.Level)
			} else {
				assert.Nil(t, result, "should not find section")
			}
		})
	}
}

func TestGetParentSection(t *testing.T) {
	sections := []Section{
		{
			Title: "Overview",
			Level: 2,
			Subsections: []Section{
				{Title: "Compatibility", Level: 3},
				{Title: "How it works", Level: 3},
			},
		},
		{
			Title: "Deployment",
			Level: 2,
			Subsections: []Section{
				{Title: "Prerequisites", Level: 3},
			},
		},
	}

	tests := []struct {
		name            string
		subsectionTitle string
		expectedParent  string
		shouldFind      bool
	}{
		{"find parent of compatibility", "Compatibility", "Overview", true},
		{"find parent of how it works", "How it works", "Overview", true},
		{"find parent of prerequisites", "Prerequisites", "Deployment", true},
		{"top-level has no parent", "Overview", "", false},
		{"non-existent subsection", "NonExistent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := parsing.GetParentSection(sections, tt.subsectionTitle)
			if tt.shouldFind {
				require.NotNil(t, parent, "should find parent")
				assert.Equal(t, tt.expectedParent, parent.Title)
			} else {
				assert.Nil(t, parent, "should not find parent")
			}
		})
	}
}

func TestParseSections_PreserveBlocks(t *testing.T) {
	content := `## Section 1

Content before preserve.

<!-- PRESERVE START -->
Preserved content here.
<!-- PRESERVE END -->

Content after preserve.

### Subsection

Subsection content.

## Section 2

More content.`

	sections := parsing.ParseSections(content)

	require.Len(t, sections, 2)
	assert.True(t, sections[0].HasPreserve, "Section 1 should have preserve block")
	assert.Contains(t, sections[0].PreserveContent, "Preserved content here")
	assert.False(t, sections[1].HasPreserve, "Section 2 should not have preserve block")
}

func TestParseSections_EdgeCases(t *testing.T) {
	t.Run("document with only subsections", func(t *testing.T) {
		content := `### Orphaned Subsection

This starts with level 3.`

		sections := parsing.ParseSections(content)

		// Should still parse, but subsection becomes a top-level item
		// (This is an edge case - ideally shouldn't happen, but parser should handle it gracefully)
		assert.Len(t, sections, 1)
	})

	t.Run("empty content", func(t *testing.T) {
		content := ""
		sections := parsing.ParseSections(content)
		assert.Len(t, sections, 0)
	})

	t.Run("no headers", func(t *testing.T) {
		content := "Just some text without headers"
		sections := parsing.ParseSections(content)
		assert.Len(t, sections, 0)
	})
}
