// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package prompts

import (
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/agents/validators"
)

// sectionInstructions contains section-specific generation instructions.
// Keys are lowercase section titles for case-insensitive matching.
// IMPORTANT: Each section includes a heading level reminder to prevent H2/H3 confusion.
var sectionInstructions = map[string]string{
	"overview": `HEADING LEVEL: This is a MAIN section - use "## Overview" (H2, two #)
OVERVIEW SECTION REQUIREMENTS:
- Write 1-2 sentences ONLY summarizing what data this integration collects.
- Use format: "The {Title} integration for Elastic enables collection of..."
- DO NOT include use cases, bullet lists, or detailed feature descriptions in the overview paragraph.
- Use cases belong ONLY in the "Supported use cases" subsection under "What data does this integration collect?"
- MUST include these subsections (H3, three #):
  ### Compatibility
  - List compatible versions of the 3rd party software/hardware
  - Include tested Elastic Stack version compatibility. IMPORTANT: Use "or later" rather than "or higher" or "or newer".

  ### How it works
  - Explain the data collection method (syslog, API, log files, etc.)
  - Describe the data flow from source to Elastic

REMINDER: Apply the CRITICAL FORMATTING RULES from the style guide.`,

	"what data does this integration collect?": `HEADING LEVEL: This is a MAIN section - use "## What data does this integration collect?" (H2, two #)
DATA COLLECTION SECTION REQUIREMENTS:
- List specific data types collected (logs, metrics, traces)
- Use bullet points to enumerate data types with brief descriptions
- Start every list with an introductory sentence ending with a colon
- MUST include this subsection (H3, three #):
  ### Supported use cases
  - List 3-5 key use cases this integration enables
  - Focus on value to the user (threat detection, compliance, monitoring)
  - Be specific: "Real-time threat detection" not just "security"

REMINDER: Apply the CRITICAL FORMATTING RULES from the style guide.
- Use monospace for data stream names: {backquote}audit{backquote}, {backquote}log{backquote}`,

	"what do i need to use this integration?": `HEADING LEVEL: This is a MAIN section - use "## What do I need to use this integration?" (H2, two #)
PREREQUISITES SECTION REQUIREMENTS:
- List ALL prerequisites before installation
- Use ### headings (H3, three #) for prerequisite categories - NEVER use #### directly under ##

EXAMPLE STRUCTURE (follow this pattern):
  ### General prerequisites
  - An active Elastic deployment
  - Elastic Agent installed...
  - Administrative access to...

  ### For audit log collection
  - Specific requirements for this data stream...

  ### For metrics collection
  - Specific requirements for this data stream...

CONTENT TO INCLUDE:
- Vendor-side requirements: admin access, API keys, network connectivity
- Elastic-side requirements: Fleet/Agent, subscriptions

CONTENT TO EXCLUDE:
- ElasticStack version requirements

HEADING HIERARCHY RULES (CRITICAL):
- Main section uses ## (H2)
- Subsections MUST use ### (H3) - never skip from ## to ####
- Sub-subsections (if needed) use #### (H4) under ### headings
- WRONG: "## Section" followed by "#### Subsection" (skips H3)
- RIGHT: "## Section" followed by "### Subsection"

FORMATTING RULES:
- NEVER use "**bold pseudo-headers**" - use ### headings instead
- Plain text for list items, never bold`,

	"how do i deploy this integration?": `HEADING LEVEL: This is a MAIN section - use "## How do I deploy this integration?" (H2, two #)
DEPLOYMENT SECTION REQUIREMENTS:
- MUST include these subsections in order (H3, three #):

  ### Agent-based deployment
  - Standard Elastic Agent installation guidance
  - Link to installation instructions
  - For network requirements, use "#### Network requirements" heading, NOT "**Network requirements**"

  ### Set up steps in {Product}
  - VENDOR-SIDE configuration steps
  - Numbered steps with specific UI paths (e.g., "Navigate to **Settings** > **Logging**")
  - For options/methods, use headings: "##### Option 1: File method" NOT "**Option 1: File method**"
  - Include CLI commands where applicable
  - Show example configurations
  - End with a vendor resources subsection using H4:
    #### Vendor resources
    - Add links from the service_info "Vendor set up resources" section

  ### Set up steps in Kibana
  - Elastic-side configuration steps
  - Navigate to Integrations, search, add
  - When multiple input types are available, guide users to "choose the setup instructions that match your configuration" NOT "choose your destination" (destination was set in vendor config)
  - For input descriptions, use plain text NOT bold: "Audit logs (file)" not "**Audit logs (file)**"

  ### Validation
  - Steps to verify data is flowing - use plain numbered lists, NOT bold
  - WRONG: "1. **Verify agent status**" - never bold numbered list items
  - RIGHT: "1. Verify agent status" - plain text for steps
  - Check Discover for data using ACTUAL data stream dataset names (see DATA STREAMS FOR VALIDATION below)
  - DO NOT assume dataset names - use the exact names provided

FORMATTING REMINDERS FOR THIS SECTION:
- Use #### or ##### headings for subsection titles, NOT **bold pseudo-headers**
- Numbered validation steps must NOT have bold: "1. Verify status" not "1. **Verify status**"
- Input type descriptions must be plain text: "Audit logs (file)" not "**Audit logs (file)**"`,

	"troubleshooting": `HEADING LEVEL: This is a MAIN section - use "## Troubleshooting" (H2, two #)
TROUBLESHOOTING SECTION REQUIREMENTS:
- ALL troubleshooting content MUST be specific to THIS integration
- DO NOT include generic Elastic Agent debugging steps
- Start with a link to common Elastic ingest troubleshooting:
  "For help with Elastic ingest tools, check [Common problems](https://www.elastic.co/docs/troubleshoot/ingest/fleet/common-problems)."

STRUCTURE (use Problem-Solution bullet format, NOT tables):
  ### Common configuration issues
  Use bullet points with Problem followed by nested Solution bullets:
  - Problem description:
    - Solution step one
    - Solution step two
  - Another problem:
    - Solution for this problem

  ### Vendor resources
  Add links to vendor troubleshooting guides. Exclude this section if no vendor troubleshooting links are available.
  - Link to vendor troubleshooting guides

EXAMPLE FORMAT (follow this style):
  ### Common configuration issues

  - No data is being collected:
    - Verify network connectivity between the source and Elastic Agent host.
    - Ensure there are no firewalls or network ACLs blocking the configured port.
    - Confirm the listening port in the integration matches the destination port on the source device.
  - TCP framing issues:
    - When using TCP input with reliable syslog mode, ensure both the source and integration settings use matching framing (e.g., {backquote}rfc6587{backquote}).

WHAT TO EXCLUDE (will be rejected):
- Generic "Verify Elastic Agent health" steps
- Generic "Check integration status" steps
- Generic "Capture agent diagnostics" steps
- Any troubleshooting that applies to ALL integrations (these belong in common docs)
- Tables with Symptom | Cause | Solution columns (use bullet points instead)
- Separate per-input subsections (consolidate into Common configuration issues)

FORMATTING RULES (CRITICAL - will be rejected if violated):
- Use ### subheadings for major issue categories, not bold list items
- Vendor resources list MUST have an introductory sentence before it
- Use monospace for configuration values, file paths, and commands
- Use nested bullet points (use "- " only, not "*") for solutions, NOT tables`,

	"performance and scaling": `HEADING LEVEL: This is a MAIN section - use "## Performance and scaling" (H2, two #)
PERFORMANCE AND SCALING SECTION REQUIREMENTS:
- Provide input-specific scaling guidance based on inputs used by this integration
- Include the standard architecture link:
  "For more information on architectures that can be used for scaling this integration, check the [Ingest Architectures](https://www.elastic.co/docs/manage-data/ingest/ingest-reference-architectures) documentation."

FORMATTING RULES (CRITICAL - will be rejected if violated):
- Use #### headings for input type subsections, NOT bold text
- Use monospace for configuration settings: {backquote}harvester_limit{backquote}, {backquote}close_inactive{backquote}`,

	"reference": `HEADING LEVEL: This is a MAIN section - use "## Reference" (H2, two #)
REFERENCE SECTION REQUIREMENTS:
INCLUDE THESE SUBSECTIONS (H3, three #):
-  (Keep this section verbatim, do not modify it):
   ### Inputs used
   {{inputDocs}}

-  ### API usage (only if the integration makes use of 3rd party APIs or httpjson/cel inputs are used)
   List APIs used with links to vendor documentation- Create a ### subsection (H3, three #) for EACH data stream in the package
- ### Vendor documentation links
  Add vendor documentation links which provide useful general information about the integration. Do not include links which are specific to troubleshooting.

- For EACH data stream, use this EXACT format:

  ### {datastream_name}

  The {backquote}{datastream_name}{backquote} data stream collects {description}.

  {{event "{datastream_name}"}}

  {{fields "{datastream_name}"}}

CRITICAL RULES:
1. Replace {datastream_name} with the ACTUAL data stream name (e.g., "conn", "dns", "traffic")
2. Include {{event "name"}} ONLY if the data stream has a sample_event.json file
3. ALWAYS include {{fields "name"}} for every data stream
4. Use list_directory on "data_stream/" to discover all data stream names
5. Check each data_stream/{name}/ folder for sample_event.json`,
}

// GetSectionInstructions returns section-specific instructions for the given section title.
// If pkgCtx is provided, it can be used to customize instructions (e.g., for Reference section).
func GetSectionInstructions(sectionTitle string, pkgCtx *validators.PackageContext) string {
	titleLower := strings.ToLower(strings.TrimSpace(sectionTitle))

	// Get base instructions
	instructions, found := sectionInstructions[titleLower]
	if !found {
		return ""
	}

	// Replace {backquote} placeholder with actual backtick
	instructions = strings.ReplaceAll(instructions, "{backquote}", "`")

	// For Reference section, add data stream-specific guidance if pkgCtx is available
	if titleLower == "reference" && pkgCtx != nil && len(pkgCtx.DataStreams) > 0 {
		instructions += "\n\nDATA STREAMS IN THIS PACKAGE:\n"
		for _, ds := range pkgCtx.DataStreams {
			instructions += "- " + ds.Name
			if ds.HasExampleEvent {
				instructions += " [has sample_event.json - include {{event}}]"
			}
			instructions += "\n"
			instructions += "  Use: {{fields \"" + ds.Name + "\"}}"
			if ds.HasExampleEvent {
				instructions += " and {{event \"" + ds.Name + "\"}}"
			}
			instructions += "\n"
		}
	}

	// For deployment section, add package title and data stream info for validation
	if titleLower == "how do i deploy this integration?" && pkgCtx != nil && pkgCtx.Manifest != nil {
		instructions = strings.ReplaceAll(instructions, "{Product}", pkgCtx.Manifest.Title)

		// Add data stream info for validation section
		if len(pkgCtx.DataStreams) > 0 {
			instructions += "\n\nDATA STREAMS FOR VALIDATION:\n"
			for _, ds := range pkgCtx.DataStreams {
				dataset := ds.Dataset
				if dataset == "" {
					dataset = pkgCtx.Manifest.Name + "." + ds.Name
				}
				instructions += "- `" + dataset + "` (type: " + ds.Type + ")\n"
			}
			instructions += "\nUse these EXACT dataset names in KQL filters, not assumed names.\n"
		}
	}

	return instructions
}
