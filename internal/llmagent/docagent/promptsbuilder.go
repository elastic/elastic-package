// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/prompts"
	"github.com/elastic/elastic-package/internal/packages"
)

// buildPrompt creates a prompt based on type and context
func (d *DocumentationAgent) buildPrompt(promptType PromptType, ctx PromptContext) string {
	var promptContent string
	var formatArgs []interface{}

	switch promptType {
	case PromptTypeRevision:
		promptContent = prompts.Load(prompts.TypeRevision)
		formatArgs = d.buildRevisionPromptArgs(ctx)
	case PromptTypeSectionGeneration:
		promptContent = prompts.Load(prompts.TypeSectionGeneration)
		formatArgs = d.buildSectionGenerationPromptArgs(ctx)
	}

	return fmt.Sprintf(promptContent, formatArgs...)
}

// buildRevisionPromptArgs prepares arguments for revision prompt
func (d *DocumentationAgent) buildRevisionPromptArgs(ctx PromptContext) []interface{} {
	return []interface{}{
		ctx.TargetDocFile, // target documentation file label
		ctx.Manifest.Name,
		ctx.Manifest.Title,
		ctx.Manifest.Type,
		ctx.Manifest.Version,
		ctx.Manifest.Description,
		ctx.TargetDocFile, // file restriction directive
		ctx.TargetDocFile, // read current content directive
		ctx.TargetDocFile, // tool usage guideline
		ctx.TargetDocFile, // step 1 - read current file
		ctx.TargetDocFile, // step 7 - write documentation
		ctx.Changes,       // user-requested changes
	}
}

// buildSectionGenerationPromptArgs prepares arguments for section generation prompt
func (d *DocumentationAgent) buildSectionGenerationPromptArgs(ctx PromptContext) []interface{} {
	levelStr := "##"
	if ctx.SectionLevel == 3 {
		levelStr = "###"
	}
	levelName := "Level 2"
	if ctx.SectionLevel == 3 {
		levelName = "Level 3"
	}

	// Get section-specific instructions
	sectionInstructions := prompts.GetSectionInstructions(ctx.SectionTitle, ctx.PackageContext)
	if sectionInstructions != "" {
		sectionInstructions = fmt.Sprintf("\nSECTION-SPECIFIC REQUIREMENTS:\n%s\n\n", sectionInstructions)
	}

	return []interface{}{
		ctx.SectionTitle,         // section title in task description
		ctx.SectionLevel,         // section level number
		ctx.TargetDocFile,        // target file name
		ctx.SectionTitle,         // section title (repeated)
		levelName,                // level name (Level 2 or Level 3)
		levelStr,                 // level prefix (## or ###)
		ctx.Manifest.Name,        // package name
		ctx.Manifest.Title,       // package title
		ctx.Manifest.Type,        // package type
		ctx.Manifest.Version,     // package version
		ctx.Manifest.Description, // package description
		ctx.SectionTitle,         // section title for get_example in tool guidelines
		ctx.SectionTitle,         // section title for get_service_info in tool guidelines
		ctx.TemplateSection,      // template section content
		sectionInstructions,      // section-specific instructions
		ctx.SectionTitle,         // section title for get_example in step 1
		ctx.SectionTitle,         // section title for get_service_info in step 3
		levelStr,                 // level prefix for step 4
		ctx.SectionTitle,         // section title for step 4
		levelStr,                 // level prefix for step 5
		ctx.SectionTitle,         // section title for step 5
	}
}

// Helper to create context with service info
func (d *DocumentationAgent) createPromptContext(manifest *packages.PackageManifest, changes string) PromptContext {
	return PromptContext{
		Manifest:      manifest,
		TargetDocFile: d.targetDocFile,
		Changes:       changes,
	}
}
