// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
)

// sectionInstructions contains section-specific generation instructions.
// Keys are lowercase section titles for case-insensitive matching.
var sectionInstructions = map[string]string{
	"overview": `OVERVIEW SECTION REQUIREMENTS:
- Start with a summary of what data this integration collects and what use cases it enables
- Use format: "The {Title} integration for Elastic enables collection of..."
- Include what the integration facilitates (security monitoring, traffic analysis, etc.)
- MUST include these subsections:
  ### Compatibility
  - List compatible versions of the 3rd party software/hardware
  - Include tested Elastic Stack version compatibility
  
  ### How it works
  - Explain the data collection method (syslog, API, log files, etc.)
  - Describe the data flow from source to Elastic`,

	"what data does this integration collect?": `DATA COLLECTION SECTION REQUIREMENTS:
- List specific data types collected (logs, metrics, traces)
- Use bullet points to enumerate data types with brief descriptions
- MUST include this subsection:
  ### Supported use cases
  - List 3-5 key use cases this integration enables
  - Focus on value to the user (threat detection, compliance, monitoring)
  - Be specific: "Real-time threat detection" not just "security"`,

	"what do i need to use this integration?": `PREREQUISITES SECTION REQUIREMENTS:
- List ALL prerequisites before installation
- Include vendor-side requirements:
  - Admin access/credentials needed
  - API keys or tokens required
  - Network connectivity requirements
- Include Elastic-side requirements:
  - Elastic Stack version
  - Fleet/Agent requirements
  - Any required subscriptions/licenses`,

	"how do i deploy this integration?": `DEPLOYMENT SECTION REQUIREMENTS:
- MUST include these subsections in order:

  ### Agent-based deployment
  - Standard Elastic Agent installation guidance
  - Link to installation instructions
  - Network requirements table if applicable

  ### Onboard and configure
  - Overview of the configuration process

  ### Set up steps in {Product}
  - VENDOR-SIDE configuration steps
  - Numbered steps with specific UI paths (e.g., "Navigate to **Settings** > **Logging**")
  - Include CLI commands where applicable
  - Show example configurations

  ### Set up steps in Kibana
  - Elastic-side configuration steps
  - Navigate to Integrations, search, add
  - Document each configuration field

  ### Validation
  - Steps to verify data is flowing
  - Check Fleet agent status
  - Check Discover for data
  - Verify dashboards are populated`,

	"troubleshooting": `TROUBLESHOOTING SECTION REQUIREMENTS:
- Format as Issue / Solution pairs
- Include common issues specific to this integration
- Include subsections for different issue types:
  ### General debugging steps
  - Agent health verification
  - Integration status check
  - Diagnostics collection
  
  ### Vendor-specific issues
  - Issues specific to the source system
  
  ### {Input type} input troubleshooting
  - Add a subsection for each input type used (TCP, UDP, API, etc.)
  - Include troubleshooting tables with: Symptom | Cause | Solution`,

	"performance and scaling": `PERFORMANCE AND SCALING SECTION REQUIREMENTS:
- Provide input-specific scaling guidance based on inputs used:
  - TCP: fault tolerance, load balancing
  - UDP: data loss warnings, buffer sizing
  - HTTP/API: rate limiting, polling intervals
  - File: harvester limits, file rotation
- Include the standard architecture link:
  "For more information on architectures that can be used for scaling this integration, check the [Ingest Architectures](https://www.elastic.co/docs/manage-data/ingest/ingest-reference-architectures) documentation."`,

	"reference": `REFERENCE SECTION REQUIREMENTS:
- Create a ### subsection for EACH data stream in the package
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
5. Check each data_stream/{name}/ folder for sample_event.json

ALSO INCLUDE:
  ### Inputs used
  {{inputDocs}}

  ### API usage (only if httpjson/cel inputs are used)
  List APIs used with links to vendor documentation`,
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

	// For deployment section, add package title if available
	if titleLower == "how do i deploy this integration?" && pkgCtx != nil && pkgCtx.Manifest != nil {
		instructions = strings.ReplaceAll(instructions, "{Product}", pkgCtx.Manifest.Title)
	}

	return instructions
}

// HasSectionInstructions returns true if there are specific instructions for this section.
func HasSectionInstructions(sectionTitle string) bool {
	titleLower := strings.ToLower(strings.TrimSpace(sectionTitle))
	_, found := sectionInstructions[titleLower]
	return found
}

