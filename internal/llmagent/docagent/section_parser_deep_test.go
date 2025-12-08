// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"testing"

	"github.com/elastic/elastic-package/internal/packages/archetype"
	"github.com/stretchr/testify/assert"
)

func TestParseSections_DeepNesting(t *testing.T) {
	t.Run("handles 4-level nesting", func(t *testing.T) {
		content := `## Level 2 Section

Level 2 content.

### Level 3 Section

Level 3 content.

#### Level 4 Section

Level 4 content.

### Another Level 3

More level 3 content.`

		sections := ParseSections(content)

		// Should have 1 top-level section (level 2)
		assert.Len(t, sections, 1)
		assert.Equal(t, "Level 2 Section", sections[0].Title)
		assert.Equal(t, 2, sections[0].Level)

		// Should have 2 level 3 subsections
		assert.Len(t, sections[0].Subsections, 2)
		assert.Equal(t, "Level 3 Section", sections[0].Subsections[0].Title)
		assert.Equal(t, 3, sections[0].Subsections[0].Level)
		assert.Equal(t, "Another Level 3", sections[0].Subsections[1].Title)
		assert.Equal(t, 3, sections[0].Subsections[1].Level)

		// First level 3 should have 1 level 4 subsection
		assert.Len(t, sections[0].Subsections[0].Subsections, 1)
		assert.Equal(t, "Level 4 Section", sections[0].Subsections[0].Subsections[0].Title)
		assert.Equal(t, 4, sections[0].Subsections[0].Subsections[0].Level)
	})

	t.Run("handles 5-level nesting", func(t *testing.T) {
		content := `## Level 2

### Level 3

#### Level 4

##### Level 5

Deep content.`

		sections := ParseSections(content)

		assert.Len(t, sections, 1)
		assert.Equal(t, 2, sections[0].Level)
		assert.Len(t, sections[0].Subsections, 1)
		assert.Equal(t, 3, sections[0].Subsections[0].Level)
		assert.Len(t, sections[0].Subsections[0].Subsections, 1)
		assert.Equal(t, 4, sections[0].Subsections[0].Subsections[0].Level)
		assert.Len(t, sections[0].Subsections[0].Subsections[0].Subsections, 1)
		assert.Equal(t, 5, sections[0].Subsections[0].Subsections[0].Subsections[0].Level)
		assert.Contains(t, sections[0].Subsections[0].Subsections[0].Subsections[0].Content, "Deep content")
	})

	t.Run("correctly pops stack when returning to higher level", func(t *testing.T) {
		content := `## Level 2A

### Level 3A

#### Level 4A

Content 4A.

### Level 3B

Content 3B (should be under 2A, not 4A).

## Level 2B

Content 2B.`

		sections := ParseSections(content)

		// Should have 2 top-level sections
		assert.Len(t, sections, 2)

		// First level 2 should have 2 level 3 sections
		assert.Len(t, sections[0].Subsections, 2)
		assert.Equal(t, "Level 3A", sections[0].Subsections[0].Title)
		assert.Equal(t, "Level 3B", sections[0].Subsections[1].Title)

		// Level 3A should have 1 level 4 section
		assert.Len(t, sections[0].Subsections[0].Subsections, 1)
		assert.Equal(t, "Level 4A", sections[0].Subsections[0].Subsections[0].Title)

		// Level 3B should NOT have any subsections (not under 4A)
		assert.Len(t, sections[0].Subsections[1].Subsections, 0)
		assert.Contains(t, sections[0].Subsections[1].Content, "Content 3B")
	})
}

func TestParseSections_MixedLevels(t *testing.T) {
	t.Run("handles skipped levels", func(t *testing.T) {
		content := `## Level 2

Content 2.

#### Level 4 (skipped level 3)

Content 4.`

		sections := ParseSections(content)

		assert.Len(t, sections, 1)
		assert.Equal(t, 2, sections[0].Level)

		// Level 4 should be a direct child of level 2 (since level 3 was skipped)
		assert.Len(t, sections[0].Subsections, 1)
		assert.Equal(t, "Level 4 (skipped level 3)", sections[0].Subsections[0].Title)
		assert.Equal(t, 4, sections[0].Subsections[0].Level)
	})
}

func TestParseSections_RealTemplate(t *testing.T) {
	t.Run("correctly parses package readme template", func(t *testing.T) {
		templateContent := archetype.GetPackageDocsReadmeTemplate()
		sections := ParseSections(templateContent)

		// Verify we have the expected top-level sections
		assert.Len(t, sections, 7)

		// Verify section titles and structure
		expectedSections := map[string]int{
			"Overview": 2, // 2 subsections: Compatibility, How it works
			"What data does this integration collect?": 1, // 1 subsection: Supported use cases
			"What do I need to use this integration?":  0, // No subsections
			"How do I deploy this integration?":        5, // 5 subsections
			"Troubleshooting":                          0, // No subsections
			"Performance and scaling":                  0, // No subsections
			"Reference":                                3, // 3 subsections
		}

		for i, section := range sections {
			t.Logf("Section %d: %q (level %d) with %d subsections",
				i, section.Title, section.Level, len(section.Subsections))

			expectedSubsections, found := expectedSections[section.Title]
			if found {
				assert.Equal(t, expectedSubsections, len(section.Subsections),
					"Section %q should have %d subsections", section.Title, expectedSubsections)
			}
		}
	})
}
