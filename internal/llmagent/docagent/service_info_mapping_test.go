// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetServiceInfoSectionsForReadmeSection(t *testing.T) {
	t.Run("returns mapped sections for Overview", func(t *testing.T) {
		sections := GetServiceInfoSectionsForReadmeSection("Overview")
		assert.Contains(t, sections, "Common use cases")
		assert.Contains(t, sections, "Data types collected")
	})

	t.Run("handles case-insensitive match", func(t *testing.T) {
		sections := GetServiceInfoSectionsForReadmeSection("overview")
		assert.Contains(t, sections, "Common use cases")
		assert.Contains(t, sections, "Data types collected")
	})

	t.Run("handles whitespace in section title", func(t *testing.T) {
		sections := GetServiceInfoSectionsForReadmeSection("  Overview  ")
		assert.Contains(t, sections, "Common use cases")
		assert.Contains(t, sections, "Data types collected")
	})

	t.Run("returns empty for unmapped section", func(t *testing.T) {
		sections := GetServiceInfoSectionsForReadmeSection("Unmapped Section Name")
		assert.Empty(t, sections)
	})

	t.Run("returns empty for non-existent section", func(t *testing.T) {
		sections := GetServiceInfoSectionsForReadmeSection("Non-existent Section")
		assert.Empty(t, sections)
	})

	t.Run("handles wildcard pattern matching", func(t *testing.T) {
		// "Set up steps in *" should match "Set up steps in NGINX"
		sections := GetServiceInfoSectionsForReadmeSection("Set up steps in NGINX")
		assert.Contains(t, sections, "Vendor set up steps")
	})

	t.Run("exact match takes precedence over wildcard", func(t *testing.T) {
		// "Set up steps in Kibana" has an exact match, should not use wildcard
		sections := GetServiceInfoSectionsForReadmeSection("Set up steps in Kibana")
		assert.Contains(t, sections, "Kibana set up steps")
		assert.NotContains(t, sections, "Vendor set up steps")
	})
}

func TestHasServiceInfoMapping(t *testing.T) {
	t.Run("returns true for mapped section", func(t *testing.T) {
		assert.True(t, HasServiceInfoMapping("Overview"))
	})

	t.Run("returns false for unmapped section", func(t *testing.T) {
		assert.False(t, HasServiceInfoMapping("Unmapped Section Name"))
	})

	t.Run("handles case-insensitive", func(t *testing.T) {
		assert.True(t, HasServiceInfoMapping("overview"))
	})
}

