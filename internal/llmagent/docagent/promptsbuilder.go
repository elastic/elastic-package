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
		formatArgs = d.buildRevisionPromptArgs(ctx).ToFormatArgs()
	case PromptTypeSectionGeneration:
		promptContent = prompts.Load(prompts.TypeSectionGeneration)
		formatArgs = d.buildSectionGenerationPromptArgs(ctx).ToFormatArgs()
	}

	return fmt.Sprintf(promptContent, formatArgs...)
}

// RevisionPromptArgs holds arguments for the revision prompt template.
type RevisionPromptArgs struct {
	TargetDocFile      string
	Manifest           *packages.PackageManifest
	FileRestriction    string
	ReadCurrentContent string
	ToolUsageGuideline string
	Step1              string
	Step7              string
	Changes            string
}

// ToFormatArgs returns the slice of values in the order expected by the revision prompt format string.
func (r RevisionPromptArgs) ToFormatArgs() []interface{} {
	m := r.Manifest
	name, title, pkgType, version, desc := "", "", "", "", ""
	if m != nil {
		name, title, pkgType, version, desc = m.Name, m.Title, m.Type, m.Version, m.Description
	}
	return []interface{}{
		r.TargetDocFile,
		name, title, pkgType, version, desc,
		r.FileRestriction,
		r.ReadCurrentContent,
		r.ToolUsageGuideline,
		r.Step1,
		r.Step7,
		r.Changes,
	}
}

// buildRevisionPromptArgs prepares arguments for revision prompt
func (d *DocumentationAgent) buildRevisionPromptArgs(ctx PromptContext) RevisionPromptArgs {
	return RevisionPromptArgs{
		TargetDocFile:      ctx.TargetDocFile,
		Manifest:           ctx.Manifest,
		FileRestriction:    ctx.TargetDocFile,
		ReadCurrentContent: ctx.TargetDocFile,
		ToolUsageGuideline: ctx.TargetDocFile,
		Step1:              ctx.TargetDocFile,
		Step7:              ctx.TargetDocFile,
		Changes:            ctx.Changes,
	}
}

// SectionGenerationPromptArgs holds arguments for the section generation prompt template.
type SectionGenerationPromptArgs struct {
	SectionTitle           string
	SectionLevel           int
	TargetDocFile          string
	LevelName              string
	LevelPrefix            string
	Manifest               *packages.PackageManifest
	GetExampleSection      string
	GetServiceInfoSection  string
	TemplateSection        string
	SectionInstructions    string
	Step1GetExampleSection string
	Step3GetServiceInfo    string
	Step4LevelPrefix       string
	Step4SectionTitle      string
	Step5LevelPrefix       string
	Step5SectionTitle      string
}

// ToFormatArgs returns the slice of values in the order expected by the section generation prompt format string.
func (s SectionGenerationPromptArgs) ToFormatArgs() []interface{} {
	m := s.Manifest
	name, title, pkgType, version, desc := "", "", "", "", ""
	if m != nil {
		name, title, pkgType, version, desc = m.Name, m.Title, m.Type, m.Version, m.Description
	}
	return []interface{}{
		s.SectionTitle,
		s.SectionLevel,
		s.TargetDocFile,
		s.SectionTitle,
		s.LevelName,
		s.LevelPrefix,
		name, title, pkgType, version, desc,
		s.GetExampleSection,
		s.GetServiceInfoSection,
		s.TemplateSection,
		s.SectionInstructions,
		s.Step1GetExampleSection,
		s.Step3GetServiceInfo,
		s.Step4LevelPrefix,
		s.Step4SectionTitle,
		s.Step5LevelPrefix,
		s.Step5SectionTitle,
	}
}

// buildSectionGenerationPromptArgs prepares arguments for section generation prompt
func (d *DocumentationAgent) buildSectionGenerationPromptArgs(ctx PromptContext) SectionGenerationPromptArgs {
	levelStr := "##"
	if ctx.SectionLevel == 3 {
		levelStr = "###"
	}
	levelName := "Level 2"
	if ctx.SectionLevel == 3 {
		levelName = "Level 3"
	}

	sectionInstructions := getSectionInstructions(ctx.SectionTitle, ctx.PackageContext)
	if sectionInstructions != "" {
		sectionInstructions = fmt.Sprintf("\nSECTION-SPECIFIC REQUIREMENTS:\n%s\n\n", sectionInstructions)
	}

	return SectionGenerationPromptArgs{
		SectionTitle:           ctx.SectionTitle,
		SectionLevel:           ctx.SectionLevel,
		TargetDocFile:          ctx.TargetDocFile,
		LevelName:              levelName,
		LevelPrefix:            levelStr,
		Manifest:               ctx.Manifest,
		GetExampleSection:      ctx.SectionTitle,
		GetServiceInfoSection:  ctx.SectionTitle,
		TemplateSection:        ctx.TemplateSection,
		SectionInstructions:    sectionInstructions,
		Step1GetExampleSection: ctx.SectionTitle,
		Step3GetServiceInfo:    ctx.SectionTitle,
		Step4LevelPrefix:       levelStr,
		Step4SectionTitle:      ctx.SectionTitle,
		Step5LevelPrefix:       levelStr,
		Step5SectionTitle:      ctx.SectionTitle,
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
