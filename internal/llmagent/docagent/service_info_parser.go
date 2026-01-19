// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/parsing"
)

// ServiceInfoManager manages the loading and retrieval of service_info.md sections
type ServiceInfoManager struct {
	packageRoot string
	sections    []Section
	loaded      bool
}

// NewServiceInfoManager creates a new ServiceInfoManager for the given package root
func NewServiceInfoManager(packageRoot string) *ServiceInfoManager {
	return &ServiceInfoManager{
		packageRoot: packageRoot,
		sections:    []Section{},
		loaded:      false,
	}
}

// Load reads and parses the service_info.md file
// Returns error if file doesn't exist or can't be parsed
func (s *ServiceInfoManager) Load() error {
	serviceInfoPath := filepath.Join(s.packageRoot, "docs", "knowledge_base", "service_info.md")

	content, err := os.ReadFile(serviceInfoPath)
	if err != nil {
		return fmt.Errorf("failed to read service_info.md: %w", err)
	}

	// Parse the content into sections
	s.sections = parsing.ParseSections(string(content))
	s.loaded = true

	return nil
}

// GetSections returns the combined content of specified sections by title
// Returns empty string if no matching sections found
func (s *ServiceInfoManager) GetSections(sectionTitles []string) string {
	if !s.loaded || len(sectionTitles) == 0 {
		return ""
	}

	var matchedSections []string

	for _, requestedTitle := range sectionTitles {
		// Try to find the section (case-insensitive, fuzzy match)
		section := parsing.FindSectionByTitleHierarchical(s.sections, requestedTitle)
		if section != nil {
			// Use GetAllContent to include subsections
			matchedSections = append(matchedSections, section.GetAllContent())
		}
	}

	if len(matchedSections) == 0 {
		return ""
	}

	// Join sections with double newline separator
	return strings.Join(matchedSections, "\n\n")
}

// GetAllSections returns the entire service_info content
func (s *ServiceInfoManager) GetAllSections() string {
	if !s.loaded {
		return ""
	}

	// Combine all top-level sections using CombineSections
	return parsing.CombineSections(s.sections)
}

// IsAvailable checks if service_info.md has been successfully loaded
func (s *ServiceInfoManager) IsAvailable() bool {
	return s.loaded
}

// GetSectionTitles returns a list of all section titles (for debugging/info)
func (s *ServiceInfoManager) GetSectionTitles() []string {
	if !s.loaded {
		return []string{}
	}

	var titles []string
	for _, section := range s.sections {
		titles = append(titles, section.Title)
		// Include subsection titles as well
		for _, subsection := range section.Subsections {
			titles = append(titles, subsection.Title)
		}
	}

	return titles
}
