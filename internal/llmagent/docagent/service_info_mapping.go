// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"strings"
)

// ServiceInfoSectionMapping defines which service_info sections are relevant
// for each README section being generated.
// The keys are README section titles, and the values are arrays of service_info section titles.
var ServiceInfoSectionMapping = map[string][]string{
	"Overview": {
		"Common use cases",
		"Data types collected",
		"Vendor Resources",
		"Documentation sites",
	},
	"Compatibility": {
		"Compatibility",
	},
	"How it works": {
		"Data types collected",
		"Vendor prerequisites",
		"Elastic prerequisites",
		"Vendor set up steps",
		"Kibana set up steps",
		"Validation steps",
	},
	"What data does this integration collect?": {
		"Data types collected",
	},
	"Supported use cases": {
		"Supported use cases",
		"Data types collected",
		"Vendor Resources",
	},
	"What do I need to use this integration?": {
		"Vendor prerequisites",
		"Elastic prerequisites",
	},
	"How do I deploy this integration?": {
		"Vendor set up steps",
		"Kibana set up steps",
	},
	"Onboard and configure": {
		"Vendor set up steps",
		"Kibana set up steps",
	},
	"Set up steps in *": {
		"Vendor set up steps",
	},
	"Set up steps in Kibana": {
		"Kibana set up steps",
	},
	"Validation Steps": {
		"Validation Steps",
	},
	"Troubleshooting": {
		"Troubleshooting",
	},
	"Performance and scaling": {
		"Performance and scaling",
	},
	"Reference": {
		"Documentation sites",
		"Vendor Resources",
		"Data types collected",
	},
	// Add more mappings as needed in the future
}

// GetServiceInfoSectionsForReadmeSection returns the relevant service_info
// section titles for a given README section title.
// Returns an empty slice if no mapping exists for the given README section.
// Comparison is case-insensitive and supports glob-style wildcards:
//   - "Pattern *" matches any string starting with "Pattern "
func GetServiceInfoSectionsForReadmeSection(readmeSectionTitle string) []string {
	// Normalize the title for lookup (case-insensitive, trim spaces)
	lowerTitle := strings.ToLower(strings.TrimSpace(readmeSectionTitle))

	// First pass: try exact matches (non-wildcard keys)
	for key, sections := range ServiceInfoSectionMapping {
		if !strings.HasSuffix(key, "*") {
			if strings.ToLower(key) == lowerTitle {
				return sections
			}
		}
	}

	// Second pass: try wildcard matches (keys ending with *)
	for key, sections := range ServiceInfoSectionMapping {
		if strings.HasSuffix(key, "*") {
			// Remove the * and check if the title starts with the prefix
			prefix := strings.ToLower(strings.TrimSuffix(key, "*"))
			if strings.HasPrefix(lowerTitle, prefix) {
				return sections
			}
		}
	}

	// No mapping found
	return []string{}
}

// HasServiceInfoMapping checks if a README section has a service_info mapping
func HasServiceInfoMapping(readmeSectionTitle string) bool {
	return len(GetServiceInfoSectionsForReadmeSection(readmeSectionTitle)) > 0
}
