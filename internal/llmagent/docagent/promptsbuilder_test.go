// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/prompts"
	"github.com/elastic/elastic-package/internal/packages"
)

func TestLoad(t *testing.T) {
	result := prompts.Load(prompts.TypeRevision)
	assert.NotEmpty(t, result)
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
