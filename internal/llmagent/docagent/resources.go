// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import _ "embed"

//go:embed _static/agent_instructions.md
var AgentInstructions string

//go:embed _static/revision_prompt.txt
var RevisionPrompt string

//go:embed _static/section_generation_prompt.txt
var SectionGenerationPrompt string

//go:embed _static/modification_analysis_prompt.txt
var ModificationAnalysisPrompt string

//go:embed _static/modification_prompt.txt
var ModificationPrompt string
