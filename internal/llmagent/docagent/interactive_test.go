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

func TestActionResult(t *testing.T) {
	t.Run("creates action result with continuation", func(t *testing.T) {
		result := ActionResult{
			NewPrompt:      "test prompt",
			ShouldContinue: true,
			Err:            nil,
		}

		assert.Equal(t, "test prompt", result.NewPrompt)
		assert.True(t, result.ShouldContinue)
		assert.NoError(t, result.Err)
	})

	t.Run("creates action result with error", func(t *testing.T) {
		result := ActionResult{
			NewPrompt:      "",
			ShouldContinue: false,
			Err:            assert.AnError,
		}

		assert.Empty(t, result.NewPrompt)
		assert.False(t, result.ShouldContinue)
		assert.Error(t, result.Err)
	})
}

func TestHandleReadmeUpdate(t *testing.T) {
	tempDir := t.TempDir()
	packageRoot := tempDir
	targetDocFile := "README.md"

	// Create _dev/build/docs directory structure
	docsDir := filepath.Join(packageRoot, "_dev", "build", "docs")
	err := os.MkdirAll(docsDir, 0o755)
	require.NoError(t, err)

	docPath := filepath.Join(docsDir, targetDocFile)

	t.Run("detects updated readme with new content", func(t *testing.T) {
		agent := &DocumentationAgent{
			packageRoot:           packageRoot,
			targetDocFile:         targetDocFile,
			originalReadmeContent: nil, // No original content
		}

		// Write new content
		err := os.WriteFile(docPath, []byte("# New Documentation\n\nThis is new content."), 0o644)
		require.NoError(t, err)

		updated, err := agent.handleReadmeUpdate()
		assert.NoError(t, err)
		assert.True(t, updated)

		// Cleanup
		os.Remove(docPath)
	})

	t.Run("detects no update when readme is empty", func(t *testing.T) {
		originalContent := "# Original content"
		agent := &DocumentationAgent{
			packageRoot:           packageRoot,
			targetDocFile:         targetDocFile,
			originalReadmeContent: &originalContent,
		}

		// Write empty content (this is considered an update from original, but empty)
		err := os.WriteFile(docPath, []byte(""), 0o644)
		require.NoError(t, err)

		updated, err := agent.handleReadmeUpdate()
		assert.Error(t, err)
		assert.False(t, updated)
		assert.Contains(t, err.Error(), "readme file empty")

		// Cleanup
		os.Remove(docPath)
	})

	t.Run("detects update when content changed from original", func(t *testing.T) {
		originalContent := "# Original Documentation"
		agent := &DocumentationAgent{
			packageRoot:           packageRoot,
			targetDocFile:         targetDocFile,
			originalReadmeContent: &originalContent,
		}

		// Write updated content
		err := os.WriteFile(docPath, []byte("# Updated Documentation\n\nNew content added."), 0o644)
		require.NoError(t, err)

		updated, err := agent.handleReadmeUpdate()
		assert.NoError(t, err)
		assert.True(t, updated)

		// Cleanup
		os.Remove(docPath)
	})

	t.Run("detects no update when content unchanged", func(t *testing.T) {
		originalContent := "# Unchanged Documentation"
		agent := &DocumentationAgent{
			packageRoot:           packageRoot,
			targetDocFile:         targetDocFile,
			originalReadmeContent: &originalContent,
		}

		// Write same content
		err := os.WriteFile(docPath, []byte(originalContent), 0o644)
		require.NoError(t, err)

		updated, err := agent.handleReadmeUpdate()
		assert.NoError(t, err)
		assert.False(t, updated)

		// Cleanup
		os.Remove(docPath)
	})
}

func TestHandleUserAction(t *testing.T) {
	tempDir := t.TempDir()
	packageRoot := tempDir
	targetDocFile := "README.md"

	// Create _dev/build/docs directory structure
	docsDir := filepath.Join(packageRoot, "_dev", "build", "docs")
	err := os.MkdirAll(docsDir, 0o755)
	require.NoError(t, err)

	t.Run("handles cancel action", func(t *testing.T) {
		agent := &DocumentationAgent{
			packageRoot:           packageRoot,
			targetDocFile:         targetDocFile,
			originalReadmeContent: nil,
		}

		result := agent.handleUserAction(ActionCancel, false)

		assert.Empty(t, result.NewPrompt)
		assert.False(t, result.ShouldContinue)
		assert.NoError(t, result.Err)
	})

	t.Run("handles unknown action", func(t *testing.T) {
		agent := &DocumentationAgent{
			packageRoot:   packageRoot,
			targetDocFile: targetDocFile,
		}

		result := agent.handleUserAction("UnknownAction", false)

		assert.Empty(t, result.NewPrompt)
		assert.False(t, result.ShouldContinue)
		assert.Error(t, result.Err)
		assert.Contains(t, result.Err.Error(), "unknown action")
	})
}

func TestHandleAcceptAction(t *testing.T) {
	tempDir := t.TempDir()
	packageRoot := tempDir
	targetDocFile := "README.md"

	// Create _dev/build/docs directory structure
	docsDir := filepath.Join(packageRoot, "_dev", "build", "docs")
	err := os.MkdirAll(docsDir, 0o755)
	require.NoError(t, err)

	docPath := filepath.Join(docsDir, targetDocFile)

	t.Run("accepts when readme is updated", func(t *testing.T) {
		agent := &DocumentationAgent{
			packageRoot:           packageRoot,
			targetDocFile:         targetDocFile,
			originalReadmeContent: nil,
		}

		result := agent.handleAcceptAction(true)

		assert.Empty(t, result.NewPrompt)
		assert.False(t, result.ShouldContinue)
		assert.NoError(t, result.Err)
	})

	t.Run("warns when preserved sections not kept", func(t *testing.T) {
		originalContent := "# Original\n<!-- PRESERVE START -->\nImportant content\n<!-- PRESERVE END -->"
		agent := &DocumentationAgent{
			packageRoot:           packageRoot,
			targetDocFile:         targetDocFile,
			originalReadmeContent: &originalContent,
		}

		// Write new content without preserved section
		err := os.WriteFile(docPath, []byte("# New Content\nNo preserved section"), 0o644)
		require.NoError(t, err)

		result := agent.handleAcceptAction(true)

		assert.False(t, result.ShouldContinue)
		assert.Error(t, result.Err)
		assert.Contains(t, result.Err.Error(), "human-edited sections not preserved")

		// Cleanup
		os.Remove(docPath)
	})

	t.Run("accepts when preserved sections are kept", func(t *testing.T) {
		originalContent := "# Original\n<!-- PRESERVE START -->\nImportant content\n<!-- PRESERVE END -->"
		agent := &DocumentationAgent{
			packageRoot:           packageRoot,
			targetDocFile:         targetDocFile,
			originalReadmeContent: &originalContent,
		}

		// Write new content with preserved section
		newContent := "# Updated\n<!-- PRESERVE START -->\nImportant content\n<!-- PRESERVE END -->\nNew info"
		err := os.WriteFile(docPath, []byte(newContent), 0o644)
		require.NoError(t, err)

		result := agent.handleAcceptAction(true)

		assert.False(t, result.ShouldContinue)
		assert.NoError(t, result.Err)

		// Cleanup
		os.Remove(docPath)
	})
}

func TestActionConstants(t *testing.T) {
	t.Run("action constants are defined", func(t *testing.T) {
		assert.Equal(t, "Accept and finalize", ActionAccept)
		assert.Equal(t, "Request changes", ActionRequest)
		assert.Equal(t, "Cancel", ActionCancel)
		assert.Equal(t, "Try again", ActionTryAgain)
		assert.Equal(t, "Exit", ActionExit)
	})
}
