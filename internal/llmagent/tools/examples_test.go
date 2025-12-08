// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetExampleHandler(t *testing.T) {
	handler := getExampleHandler()

	tests := []struct {
		name         string
		exampleName  string
		section      string
		expectError  bool
		checkContent func(t *testing.T, content string)
	}{
		{
			name:        "get entire example file",
			exampleName: "fortinet_fortigate.md",
			section:     "",
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "# Fortinet FortiGate")
				assert.Contains(t, content, "## Overview")
				assert.Contains(t, content, "## Troubleshooting")
			},
		},
		{
			name:        "get specific section",
			exampleName: "fortinet_fortigate.md",
			section:     "Overview",
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "## Overview")
				assert.Contains(t, content, "FortiGate")
			},
		},
		{
			name:        "get Troubleshooting section",
			exampleName: "fortinet_fortigate.md",
			section:     "Troubleshooting",
			checkContent: func(t *testing.T, content string) {
				assert.Contains(t, content, "## Troubleshooting")
			},
		},
		{
			name:        "non-existent file",
			exampleName: "non_existent.md",
			expectError: true,
		},
		{
			name:        "non-existent section",
			exampleName: "fortinet_fortigate.md",
			section:     "Non Existent Section",
			expectError: true,
		},
		{
			name:        "empty name",
			exampleName: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler(nil, GetExampleArgs{Name: tt.exampleName, Section: tt.section})
			require.NoError(t, err) // Handler shouldn't return Go error, uses result.Error

			if tt.expectError {
				assert.NotEmpty(t, result.Error)
			} else {
				assert.Empty(t, result.Error)
				if tt.checkContent != nil {
					tt.checkContent(t, result.Content)
				}
			}
		})
	}
}

func TestGetDefaultExampleContent(t *testing.T) {
	content := GetDefaultExampleContent()

	assert.NotEmpty(t, content)
	assert.Contains(t, content, "# Fortinet FortiGate")
	assert.Contains(t, content, "## Overview")
}

func TestGetExampleContent(t *testing.T) {
	t.Run("full file", func(t *testing.T) {
		content, err := GetExampleContent("fortinet_fortigate.md", "")
		require.NoError(t, err)
		assert.Contains(t, content, "# Fortinet FortiGate")
	})

	t.Run("specific section", func(t *testing.T) {
		content, err := GetExampleContent("fortinet_fortigate.md", "Overview")
		require.NoError(t, err)
		assert.Contains(t, content, "## Overview")
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := GetExampleContent("non_existent.md", "")
		assert.Error(t, err)
	})

	t.Run("non-existent section", func(t *testing.T) {
		_, err := GetExampleContent("fortinet_fortigate.md", "Non Existent")
		assert.Error(t, err)
	})
}

func TestParseExampleSections(t *testing.T) {
	content := `# Title

## Section One

Content for section one.

### Subsection One A

Subsection content.

## Section Two

Content for section two.
`

	sections := parseExampleSections(content)

	// Should have 2 top-level sections (## headers)
	require.Len(t, sections, 2)

	// Check first section
	assert.Equal(t, "Section One", sections[0].Title)
	assert.Equal(t, 2, sections[0].Level)
	assert.Len(t, sections[0].Subsections, 1)
	assert.Equal(t, "Subsection One A", sections[0].Subsections[0].Title)

	// Check second section
	assert.Equal(t, "Section Two", sections[1].Title)
	assert.Equal(t, 2, sections[1].Level)
}

func TestFindSectionByTitle(t *testing.T) {
	sections := []*exampleSection{
		{
			Title: "Overview",
			Level: 2,
			Subsections: []*exampleSection{
				{Title: "How it works", Level: 3},
			},
		},
		{
			Title: "Configuration",
			Level: 2,
		},
	}

	t.Run("find top-level section", func(t *testing.T) {
		sec := findSectionByTitle(sections, "Overview")
		require.NotNil(t, sec)
		assert.Equal(t, "Overview", sec.Title)
	})

	t.Run("find subsection", func(t *testing.T) {
		sec := findSectionByTitle(sections, "How it works")
		require.NotNil(t, sec)
		assert.Equal(t, "How it works", sec.Title)
	})

	t.Run("case insensitive", func(t *testing.T) {
		sec := findSectionByTitle(sections, "OVERVIEW")
		require.NotNil(t, sec)
		assert.Equal(t, "Overview", sec.Title)
	})

	t.Run("fuzzy match", func(t *testing.T) {
		sec := findSectionByTitle(sections, "config")
		require.NotNil(t, sec)
		assert.Equal(t, "Configuration", sec.Title)
	})

	t.Run("not found", func(t *testing.T) {
		sec := findSectionByTitle(sections, "nonexistent")
		assert.Nil(t, sec)
	})
}

func TestCreateExampleTools(t *testing.T) {
	tools := CreateExampleTools()

	require.Len(t, tools, 2)
	assert.Equal(t, "list_examples", tools[0].Name())
	assert.Equal(t, "get_example", tools[1].Name())
}
