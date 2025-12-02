// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceInfoManager_Load(t *testing.T) {
	t.Run("loads existing service_info file", func(t *testing.T) {
		tmpDir := t.TempDir()
		serviceInfoDir := filepath.Join(tmpDir, "docs", "knowledge_base")
		require.NoError(t, os.MkdirAll(serviceInfoDir, 0o755))

		serviceInfoPath := filepath.Join(serviceInfoDir, "service_info.md")
		content := `## Common use cases

This service is used for logging.

## Data types collected

Logs and metrics.`

		require.NoError(t, os.WriteFile(serviceInfoPath, []byte(content), 0o644))

		manager := NewServiceInfoManager(tmpDir)
		err := manager.Load()
		
		assert.NoError(t, err)
		assert.True(t, manager.IsAvailable())
		assert.Len(t, manager.sections, 2)
	})

	t.Run("returns error when file doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		manager := NewServiceInfoManager(tmpDir)
		err := manager.Load()
		
		assert.Error(t, err)
		assert.False(t, manager.IsAvailable())
	})
}

func TestServiceInfoManager_GetSections(t *testing.T) {
	tmpDir := t.TempDir()
	serviceInfoDir := filepath.Join(tmpDir, "docs", "knowledge_base")
	require.NoError(t, os.MkdirAll(serviceInfoDir, 0o755))

	serviceInfoPath := filepath.Join(serviceInfoDir, "service_info.md")
	content := `## Common use cases

This service is used for logging and monitoring.

## Data types collected

Collects logs and metrics.

## Setup instructions

Install and configure the service.`

	require.NoError(t, os.WriteFile(serviceInfoPath, []byte(content), 0o644))

	manager := NewServiceInfoManager(tmpDir)
	require.NoError(t, manager.Load())

	t.Run("retrieves single section", func(t *testing.T) {
		result := manager.GetSections([]string{"Common use cases"})
		assert.Contains(t, result, "Common use cases")
		assert.Contains(t, result, "logging and monitoring")
	})

	t.Run("retrieves multiple sections", func(t *testing.T) {
		result := manager.GetSections([]string{"Common use cases", "Data types collected"})
		assert.Contains(t, result, "Common use cases")
		assert.Contains(t, result, "Data types collected")
		assert.Contains(t, result, "logging and monitoring")
		assert.Contains(t, result, "Collects logs and metrics")
	})

	t.Run("returns empty for non-existent section", func(t *testing.T) {
		result := manager.GetSections([]string{"Non-existent Section"})
		assert.Empty(t, result)
	})

	t.Run("returns empty when not loaded", func(t *testing.T) {
		emptyManager := NewServiceInfoManager(tmpDir)
		result := emptyManager.GetSections([]string{"Common use cases"})
		assert.Empty(t, result)
	})
}

func TestServiceInfoManager_GetAllSections(t *testing.T) {
	tmpDir := t.TempDir()
	serviceInfoDir := filepath.Join(tmpDir, "docs", "knowledge_base")
	require.NoError(t, os.MkdirAll(serviceInfoDir, 0o755))

	serviceInfoPath := filepath.Join(serviceInfoDir, "service_info.md")
	content := `## Common use cases

This service is used for logging.

## Data types collected

Logs and metrics.`

	require.NoError(t, os.WriteFile(serviceInfoPath, []byte(content), 0o644))

	manager := NewServiceInfoManager(tmpDir)
	require.NoError(t, manager.Load())

	result := manager.GetAllSections()
	assert.Contains(t, result, "Common use cases")
	assert.Contains(t, result, "Data types collected")
	assert.Contains(t, result, "logging")
	assert.Contains(t, result, "Logs and metrics")
}

func TestServiceInfoManager_GetSectionTitles(t *testing.T) {
	tmpDir := t.TempDir()
	serviceInfoDir := filepath.Join(tmpDir, "docs", "knowledge_base")
	require.NoError(t, os.MkdirAll(serviceInfoDir, 0o755))

	serviceInfoPath := filepath.Join(serviceInfoDir, "service_info.md")
	content := `## Common use cases

Content 1

### Subsection

Subsection content

## Data types collected

Content 2`

	require.NoError(t, os.WriteFile(serviceInfoPath, []byte(content), 0o644))

	manager := NewServiceInfoManager(tmpDir)
	require.NoError(t, manager.Load())

	titles := manager.GetSectionTitles()
	assert.Contains(t, titles, "Common use cases")
	assert.Contains(t, titles, "Data types collected")
	assert.Contains(t, titles, "Subsection")
}

