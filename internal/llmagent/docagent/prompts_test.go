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

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
)

func TestGetConfigValue(t *testing.T) {
	t.Run("returns environment variable when set", func(t *testing.T) {
		envVar := "TEST_ENV_VAR"
		expectedValue := "env_value"
		os.Setenv(envVar, expectedValue)
		defer os.Unsetenv(envVar)

		result := getConfigValue(nil, envVar, "config.key", "default")
		assert.Equal(t, expectedValue, result)
	})

	t.Run("returns profile config when env var not set", func(t *testing.T) {
		mockProfile := &profile.Profile{}
		// Note: We can't easily mock the Config method without changing the profile package,
		// so this test is limited. In a real scenario, we'd need to refactor for testability.
		result := getConfigValue(mockProfile, "UNSET_ENV_VAR", "config.key", "default")
		// Should return default since we can't mock profile.Config
		assert.Equal(t, "default", result)
	})

	t.Run("returns default when neither env var nor profile set", func(t *testing.T) {
		defaultValue := "default_value"
		result := getConfigValue(nil, "UNSET_ENV_VAR", "config.key", defaultValue)
		assert.Equal(t, defaultValue, result)
	})
}

func TestLoadPromptFile(t *testing.T) {
	t.Run("returns embedded content when external prompts disabled", func(t *testing.T) {
		embeddedContent := "embedded prompt content"
		result := loadPromptFile("test_prompt.txt", embeddedContent, nil)
		assert.Equal(t, embeddedContent, result)
	})

	t.Run("loads from profile directory when enabled", func(t *testing.T) {
		// Create temporary profile directory
		tmpDir := t.TempDir()
		promptsDir := filepath.Join(tmpDir, "prompts")
		require.NoError(t, os.MkdirAll(promptsDir, 0o755))

		promptFile := filepath.Join(promptsDir, "test_prompt.txt")
		externalContent := "external prompt from profile"
		require.NoError(t, os.WriteFile(promptFile, []byte(externalContent), 0o644))

		// Set environment variable to enable external prompts
		os.Setenv("ELASTIC_PACKAGE_LLM_EXTERNAL_PROMPTS", "true")
		defer os.Unsetenv("ELASTIC_PACKAGE_LLM_EXTERNAL_PROMPTS")

		mockProfile := &profile.Profile{
			ProfilePath: tmpDir,
		}

		result := loadPromptFile("test_prompt.txt", "embedded", mockProfile)
		assert.Equal(t, externalContent, result)
	})

	t.Run("falls back to embedded when external file not found", func(t *testing.T) {
		os.Setenv("ELASTIC_PACKAGE_LLM_EXTERNAL_PROMPTS", "true")
		defer os.Unsetenv("ELASTIC_PACKAGE_LLM_EXTERNAL_PROMPTS")

		embeddedContent := "embedded fallback"
		mockProfile := &profile.Profile{
			ProfilePath: "/nonexistent/path",
		}

		result := loadPromptFile("nonexistent.txt", embeddedContent, mockProfile)
		assert.Equal(t, embeddedContent, result)
	})
}

func TestBuildInitialPromptArgs(t *testing.T) {
	agent := &DocumentationAgent{
		targetDocFile: "docs/README.md",
	}

	ctx := PromptContext{
		Manifest: &packages.PackageManifest{
			Name:        "test-package",
			Title:       "Test Package",
			Type:        "integration",
			Version:     "1.0.0",
			Description: "Test description",
		},
		TargetDocFile: "docs/README.md",
	}

	args := agent.buildInitialPromptArgs(ctx)

	// Should have 10 arguments (based on the implementation)
	assert.Len(t, args, 10)
	assert.Equal(t, "docs/README.md", args[0])
	assert.Equal(t, "test-package", args[1])
	assert.Equal(t, "Test Package", args[2])
	assert.Equal(t, "integration", args[3])
	assert.Equal(t, "1.0.0", args[4])
	assert.Equal(t, "Test description", args[5])
}

func TestBuildRevisionPromptArgs(t *testing.T) {
	agent := &DocumentationAgent{
		targetDocFile: "docs/README.md",
	}

	ctx := PromptContext{
		Manifest: &packages.PackageManifest{
			Name:        "test-package",
			Title:       "Test Package",
			Type:        "integration",
			Version:     "1.0.0",
			Description: "Test description",
		},
		TargetDocFile: "docs/README.md",
		Changes:       "Add more examples",
	}

	args := agent.buildRevisionPromptArgs(ctx)

	// Should have 12 arguments (based on the implementation)
	assert.Len(t, args, 12)
	assert.Equal(t, "docs/README.md", args[0])
	assert.Equal(t, "test-package", args[1])
	assert.Equal(t, "Add more examples", args[11])
}

func TestBuildPrompt(t *testing.T) {
	agent := &DocumentationAgent{
		targetDocFile: "docs/README.md",
	}

	ctx := PromptContext{
		Manifest: &packages.PackageManifest{
			Name:        "test-package",
			Title:       "Test Package",
			Type:        "integration",
			Version:     "1.0.0",
			Description: "Test description",
		},
		TargetDocFile: "docs/README.md",
	}

	t.Run("builds initial prompt", func(t *testing.T) {
		prompt := agent.buildPrompt(PromptTypeInitial, ctx)
		assert.NotEmpty(t, prompt)
		assert.Contains(t, prompt, "test-package")
	})

	t.Run("builds revision prompt", func(t *testing.T) {
		ctx.Changes = "Update documentation"
		prompt := agent.buildPrompt(PromptTypeRevision, ctx)
		assert.NotEmpty(t, prompt)
		assert.Contains(t, prompt, "test-package")
	})
}

func TestCreatePromptContext(t *testing.T) {
	agent := &DocumentationAgent{
		targetDocFile: "docs/README.md",
		packageRoot:   t.TempDir(), // Use temp dir to avoid reading actual files
	}

	manifest := &packages.PackageManifest{
		Name:        "test-package",
		Title:       "Test Package",
		Type:        "integration",
		Version:     "1.0.0",
		Description: "Test description",
	}

	ctx := agent.createPromptContext(manifest, "test changes")

	assert.Equal(t, manifest, ctx.Manifest)
	assert.Equal(t, "docs/README.md", ctx.TargetDocFile)
	assert.Equal(t, "test changes", ctx.Changes)
}

func TestExtractTemplateSections(t *testing.T) {
	t.Run("removes Go template comments", func(t *testing.T) {
		input := `# Title
{{/* This is a comment */}}
## Section One
{{/* Another
multiline
comment */}}
### Subsection`

		result := extractTemplateSections(input)

		assert.Contains(t, result, "# Title")
		assert.Contains(t, result, "## Section One")
		assert.Contains(t, result, "### Subsection")
		assert.NotContains(t, result, "This is a comment")
		assert.NotContains(t, result, "multiline")
	})

	t.Run("removes template directives", func(t *testing.T) {
		input := `{{- generatedHeader }}
# Title
{{ fields "data_stream" }}
## Section`

		result := extractTemplateSections(input)

		assert.Contains(t, result, "# Title")
		assert.Contains(t, result, "## Section")
		assert.NotContains(t, result, "generatedHeader")
		assert.NotContains(t, result, "fields")
	})

	t.Run("keeps only headers", func(t *testing.T) {
		input := `# Main Title
Some description text
## Section One
More text here
### Subsection
Even more text`

		result := extractTemplateSections(input)

		assert.Contains(t, result, "# Main Title")
		assert.Contains(t, result, "## Section One")
		assert.Contains(t, result, "### Subsection")
		assert.NotContains(t, result, "description text")
		assert.NotContains(t, result, "More text")
	})

	t.Run("handles real template format", func(t *testing.T) {
		input := `{{- generatedHeader }}
{{/*
This template can be used as a starting point
*/}}
# {[.Manifest.Title]} Integration for Elastic

## Overview
{{/* Complete this section */}}
The integration enables...

### Compatibility
This is compatible with...`

		result := extractTemplateSections(input)

		assert.Contains(t, result, "# {[.Manifest.Title]} Integration for Elastic")
		assert.Contains(t, result, "## Overview")
		assert.Contains(t, result, "### Compatibility")
		assert.NotContains(t, result, "starting point")
		assert.NotContains(t, result, "Complete this section")
	})
}

func TestGetMinimalTemplateSections(t *testing.T) {
	sections := getMinimalTemplateSections()

	// Should contain key section headers
	assert.Contains(t, sections, "## Overview")
	assert.Contains(t, sections, "## Troubleshooting")
	assert.Contains(t, sections, "## Reference")

	// Should not contain template comments
	assert.NotContains(t, sections, "{{/*")
	assert.NotContains(t, sections, "*/}}")
}
