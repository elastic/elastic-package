// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package doceval

import (
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/agents/validators"
)

// QualityMetrics holds computed quality scores for documentation
type QualityMetrics struct {
	// StructureScore (0-100): Required sections present, heading hierarchy
	StructureScore float64 `json:"structure_score"`

	// AccuracyScore (0-100): Facts match package metadata
	AccuracyScore float64 `json:"accuracy_score"`

	// CompletenessScore (0-100): All data streams documented, setup complete
	CompletenessScore float64 `json:"completeness_score"`

	// QualityScore (0-100): Writing quality, clarity
	QualityScore float64 `json:"quality_score"`

	// PlaceholderCount: Number of placeholder markers
	PlaceholderCount int `json:"placeholder_count"`

	// CompositeScore (0-100): Weighted combination of all scores
	CompositeScore float64 `json:"composite_score"`

	// Details provides breakdown information
	Details *MetricsDetails `json:"details,omitempty"`
}

// MetricsDetails provides detailed breakdown of scoring
type MetricsDetails struct {
	// Structure details
	RequiredSectionsFound int      `json:"required_sections_found"`
	RequiredSectionsTotal int      `json:"required_sections_total"`
	MissingSections       []string `json:"missing_sections,omitempty"`
	HeadingHierarchyValid bool     `json:"heading_hierarchy_valid"`

	// Accuracy details
	PackageNameFound       bool     `json:"package_name_found"`
	PackageTitleFound      bool     `json:"package_title_found"`
	DataStreamsDocumented  int      `json:"data_streams_documented"`
	DataStreamsTotal       int      `json:"data_streams_total"`
	MissingDataStreams     []string `json:"missing_data_streams,omitempty"`
	FieldReferencesValid   int      `json:"field_references_valid"`
	FieldReferencesInvalid int      `json:"field_references_invalid"`

	// Completeness details
	HasSetupSection     bool `json:"has_setup_section"`
	HasVendorSetup      bool `json:"has_vendor_setup"`
	HasKibanaSetup      bool `json:"has_kibana_setup"`
	HasValidationSteps  bool `json:"has_validation_steps"`
	HasTroubleshooting  bool `json:"has_troubleshooting"`
	HasReferenceSection bool `json:"has_reference_section"`

	// Quality details
	TodoMarkersFound      int `json:"todo_markers_found"`
	VaguePhrasesFound     int `json:"vague_phrases_found"`
	PassiveVoiceInstances int `json:"passive_voice_instances"`
	ShortSectionsCount    int `json:"short_sections_count"`

	// Content stats
	TotalWordCount int `json:"total_word_count"`
	TotalLineCount int `json:"total_line_count"`
	CodeBlockCount int `json:"code_block_count"`
}

// Weight constants for composite score calculation
const (
	WeightStructure    = 0.20
	WeightAccuracy     = 0.30
	WeightCompleteness = 0.25
	WeightQuality      = 0.15
	WeightPlaceholders = 0.10
)

// Quality scoring penalties and thresholds
const (
	todoDeductionPer         = 20.0  // Points deducted per TODO marker
	todoDeductionMax         = 40.0  // Maximum total deduction for TODOs
	vagueDeductionPer        = 5.0   // Points deducted per vague phrase
	vagueDeductionMax        = 20.0  // Maximum total deduction for vague phrases
	passiveVoiceThreshold    = 10    // Threshold for passive voice penalty
	passiveVoiceDeduction    = 10.0  // Points deducted for excessive passive voice
	shortSectionDeductionPer = 5.0   // Points deducted per short section
	shortSectionDeductionMax = 20.0  // Maximum total deduction for short sections
	minNonEmptyLines         = 3     // Minimum non-empty lines for a valid section
	placeholderPenaltyPer    = 5.0   // Points deducted per placeholder
	placeholderPenaltyMax    = 100.0 // Maximum placeholder penalty
)

// getRequiredH2Sections returns required H2 section names from validators package
func getRequiredH2Sections() []string {
	sections := make([]string, 0, len(validators.RequiredSections))
	for _, sec := range validators.RequiredSections {
		sections = append(sections, strings.ToLower(sec.Name))
	}
	return sections
}

// getRequiredH3Sections returns required H3 subsection names from validators package
func getRequiredH3Sections() []string {
	var subsections []string
	for _, sec := range validators.RequiredSections {
		for _, sub := range sec.Subsections {
			subsections = append(subsections, strings.ToLower(sub))
		}
	}
	return subsections
}

// getRecommendedSections returns recommended section names from validators package
func getRecommendedSections() []string {
	sections := make([]string, 0, len(validators.RecommendedSections))
	for _, sec := range validators.RecommendedSections {
		sections = append(sections, strings.ToLower(sec))
	}
	return sections
}

// ComputeMetrics calculates quality metrics for documentation content
func ComputeMetrics(content string, pkgCtx *validators.PackageContext) *QualityMetrics {
	metrics := &QualityMetrics{
		Details: &MetricsDetails{},
	}

	contentLower := strings.ToLower(content)

	// Compute individual scores
	metrics.StructureScore = computeStructureScore(content, metrics.Details)
	metrics.AccuracyScore = computeAccuracyScore(content, contentLower, pkgCtx, metrics.Details)
	metrics.CompletenessScore = computeCompletenessScore(contentLower, metrics.Details)
	metrics.QualityScore = computeQualityScore(content, metrics.Details)
	metrics.PlaceholderCount = countPlaceholders(content)

	// Compute composite score
	placeholderPenalty := float64(metrics.PlaceholderCount) * placeholderPenaltyPer
	if placeholderPenalty > placeholderPenaltyMax {
		placeholderPenalty = placeholderPenaltyMax
	}
	placeholderScore := 100 - placeholderPenalty

	metrics.CompositeScore = (metrics.StructureScore * WeightStructure) +
		(metrics.AccuracyScore * WeightAccuracy) +
		(metrics.CompletenessScore * WeightCompleteness) +
		(metrics.QualityScore * WeightQuality) +
		(placeholderScore * WeightPlaceholders)

	// Compute content stats
	metrics.Details.TotalWordCount = len(strings.Fields(content))
	metrics.Details.TotalLineCount = len(strings.Split(content, "\n"))
	metrics.Details.CodeBlockCount = strings.Count(content, "```") / 2

	return metrics
}

// computeStructureScore evaluates document structure
func computeStructureScore(content string, details *MetricsDetails) float64 {
	score := 0.0

	requiredH2 := getRequiredH2Sections()
	requiredH3 := getRequiredH3Sections()
	recommended := getRecommendedSections()

	// Check required H2 sections (40 points)
	h2Found := 0
	h2Total := len(requiredH2)
	for _, section := range requiredH2 {
		// Match ## followed by the section name (case insensitive)
		pattern := `(?im)^##\s+` + regexp.QuoteMeta(section)
		if regexp.MustCompile(pattern).MatchString(content) {
			h2Found++
		} else {
			details.MissingSections = append(details.MissingSections, "## "+section)
		}
	}

	// Check required H3 sections (20 points)
	h3Found := 0
	h3Total := len(requiredH3)
	for _, section := range requiredH3 {
		// Match ### followed by the section name (case insensitive)
		pattern := `(?im)^###\s+` + regexp.QuoteMeta(section)
		if regexp.MustCompile(pattern).MatchString(content) {
			h3Found++
		} else {
			details.MissingSections = append(details.MissingSections, "### "+section)
		}
	}

	// Calculate total required sections score
	details.RequiredSectionsTotal = h2Total + h3Total
	details.RequiredSectionsFound = h2Found + h3Found
	if details.RequiredSectionsTotal > 0 {
		score += (float64(details.RequiredSectionsFound) / float64(details.RequiredSectionsTotal)) * 60
	}

	// Check recommended sections (20 points)
	recommendedFound := 0
	for _, section := range recommended {
		// Check for both H2 and H3 versions
		h2Pattern := `(?im)^##\s+` + regexp.QuoteMeta(section)
		h3Pattern := `(?im)^###\s+` + regexp.QuoteMeta(section)
		if regexp.MustCompile(h2Pattern).MatchString(content) ||
			regexp.MustCompile(h3Pattern).MatchString(content) {
			recommendedFound++
		}
	}
	if len(recommended) > 0 {
		score += (float64(recommendedFound) / float64(len(recommended))) * 20
	}

	// Check heading hierarchy (20 points)
	details.HeadingHierarchyValid = checkHeadingHierarchy(content)
	if details.HeadingHierarchyValid {
		score += 20
	}

	return score
}

// checkHeadingHierarchy verifies heading levels are sequential
func checkHeadingHierarchy(content string) bool {
	headingPattern := regexp.MustCompile(`(?m)^(#{1,6})\s+`)
	matches := headingPattern.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		return false
	}

	// First heading should be H1
	if len(matches[0][1]) != 1 {
		return false
	}

	// Check for level jumps
	prevLevel := 0
	for _, match := range matches {
		level := len(match[1])
		if prevLevel > 0 && level > prevLevel+1 {
			return false
		}
		prevLevel = level
	}

	return true
}

// computeAccuracyScore evaluates content accuracy against package metadata
func computeAccuracyScore(content, contentLower string, pkgCtx *validators.PackageContext, details *MetricsDetails) float64 {
	if pkgCtx == nil || pkgCtx.Manifest == nil {
		return 50.0 // Default score when no context available
	}

	score := 0.0

	// Check package name mentioned (20 points)
	if strings.Contains(contentLower, strings.ToLower(pkgCtx.Manifest.Name)) {
		details.PackageNameFound = true
		score += 20
	}

	// Check package title mentioned (20 points)
	if pkgCtx.Manifest.Title != "" && strings.Contains(content, pkgCtx.Manifest.Title) {
		details.PackageTitleFound = true
		score += 20
	}

	// Check data streams documented (40 points)
	details.DataStreamsTotal = len(pkgCtx.DataStreams)
	for _, ds := range pkgCtx.DataStreams {
		nameMentioned := strings.Contains(contentLower, strings.ToLower(ds.Name))
		titleMentioned := ds.Title != "" && strings.Contains(contentLower, strings.ToLower(ds.Title))
		if nameMentioned || titleMentioned {
			details.DataStreamsDocumented++
		} else {
			details.MissingDataStreams = append(details.MissingDataStreams, ds.Name)
		}
	}
	if details.DataStreamsTotal > 0 {
		score += (float64(details.DataStreamsDocumented) / float64(details.DataStreamsTotal)) * 40
	} else {
		score += 40 // No data streams to document
	}

	// Check field references (20 points)
	fieldPattern := regexp.MustCompile("`([a-z][a-z0-9_\\.]+)`")
	matches := fieldPattern.FindAllStringSubmatch(content, -1)

	allFields := pkgCtx.GetAllFieldNames()
	fieldSet := make(map[string]bool)
	for _, f := range allFields {
		fieldSet[f] = true
	}

	for _, match := range matches {
		if len(match) > 1 {
			fieldRef := match[1]
			if strings.Contains(fieldRef, ".") && !isCommonNonField(fieldRef) {
				if fieldSet[fieldRef] || isECSField(fieldRef) {
					details.FieldReferencesValid++
				} else {
					details.FieldReferencesInvalid++
				}
			}
		}
	}

	totalRefs := details.FieldReferencesValid + details.FieldReferencesInvalid
	if totalRefs > 0 {
		score += (float64(details.FieldReferencesValid) / float64(totalRefs)) * 20
	} else {
		score += 20 // No field references to validate
	}

	return score
}

// isCommonNonField returns true if the string is likely not a field name
func isCommonNonField(s string) bool {
	nonFieldPatterns := []string{
		".yml", ".yaml", ".json", ".md", ".log",
		"data_stream", "_dev", "_meta", "/",
	}
	for _, pattern := range nonFieldPatterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

// isECSField returns true if the field looks like an ECS field
func isECSField(fieldRef string) bool {
	ecsPrefixes := []string{
		"event.", "host.", "agent.", "ecs.", "message.", "log.", "error.",
		"source.", "destination.", "network.", "process.", "file.", "user.",
		"url.", "http.", "dns.", "tls.", "service.", "cloud.", "container.",
		"@timestamp",
	}
	for _, prefix := range ecsPrefixes {
		if strings.HasPrefix(fieldRef, prefix) {
			return true
		}
	}
	return false
}

// computeCompletenessScore evaluates documentation completeness
func computeCompletenessScore(contentLower string, details *MetricsDetails) float64 {
	score := 0.0

	// Check setup/deployment section (25 points)
	// Template uses "How do I deploy this integration?" as the main setup section
	details.HasSetupSection = strings.Contains(contentLower, "## how do i deploy") ||
		strings.Contains(contentLower, "## setup") ||
		strings.Contains(contentLower, "## installation") ||
		strings.Contains(contentLower, "## getting started")
	if details.HasSetupSection {
		score += 25
	}

	// Check vendor setup instructions (15 points)
	vendorIndicators := []string{
		"configure", "enable logging", "syslog", "api key",
		"credentials", "prerequisite", "vendor", "service",
	}
	for _, indicator := range vendorIndicators {
		if strings.Contains(contentLower, indicator) {
			details.HasVendorSetup = true
			score += 15
			break
		}
	}

	// Check Kibana setup instructions (15 points)
	kibanaIndicators := []string{
		"kibana", "elastic agent", "fleet", "add integration",
		"enroll", "policy", "index pattern",
	}
	for _, indicator := range kibanaIndicators {
		if strings.Contains(contentLower, indicator) {
			details.HasKibanaSetup = true
			score += 15
			break
		}
	}

	// Check validation steps (15 points)
	validationIndicators := []string{
		"verify", "validate", "confirm", "check", "test",
		"discover", "data appears", "logs are", "metrics are",
	}
	for _, indicator := range validationIndicators {
		if strings.Contains(contentLower, indicator) {
			details.HasValidationSteps = true
			score += 15
			break
		}
	}

	// Check troubleshooting section (15 points)
	details.HasTroubleshooting = strings.Contains(contentLower, "## troubleshooting") ||
		strings.Contains(contentLower, "### troubleshooting")
	if details.HasTroubleshooting {
		score += 15
	}

	// Check reference section (15 points)
	details.HasReferenceSection = strings.Contains(contentLower, "## reference") ||
		strings.Contains(contentLower, "## fields") ||
		strings.Contains(contentLower, "## exported fields") ||
		strings.Contains(contentLower, "exported fields")
	if details.HasReferenceSection {
		score += 15
	}

	return score
}

// computeQualityScore evaluates writing quality
func computeQualityScore(content string, details *MetricsDetails) float64 {
	score := 100.0

	// Check for TODO markers
	todoPatterns := []string{`\bTODO\b`, `\bFIXME\b`, `\bHACK\b`, `\bTBD\b`}
	for _, pattern := range todoPatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		matches := re.FindAllString(content, -1)
		details.TodoMarkersFound += len(matches)
	}
	todoDeduction := float64(details.TodoMarkersFound) * todoDeductionPer
	if todoDeduction > todoDeductionMax {
		todoDeduction = todoDeductionMax
	}
	score -= todoDeduction

	// Check for vague phrases
	vaguePhrases := []string{
		`\bsimply\s+`, `\bjust\s+`, `\beasily\s+`,
		`\bobviously\s+`, `\bclearly\s+`,
	}
	for _, pattern := range vaguePhrases {
		re := regexp.MustCompile(`(?i)` + pattern)
		matches := re.FindAllString(content, -1)
		details.VaguePhrasesFound += len(matches)
	}
	vagueDeduction := float64(details.VaguePhrasesFound) * vagueDeductionPer
	if vagueDeduction > vagueDeductionMax {
		vagueDeduction = vagueDeductionMax
	}
	score -= vagueDeduction

	// Check for excessive passive voice
	passivePattern := regexp.MustCompile(`(?i)\b(?:is|are|was|were|be|been|being)\s+(?:configured|installed|enabled|set|defined|used|required|needed|supported)\b`)
	details.PassiveVoiceInstances = len(passivePattern.FindAllString(content, -1))
	if details.PassiveVoiceInstances > passiveVoiceThreshold {
		score -= passiveVoiceDeduction
	}

	// Check for short sections
	sectionPattern := regexp.MustCompile(`(?m)^##\s+(.+)$`)
	matches := sectionPattern.FindAllStringSubmatchIndex(content, -1)

	for i, match := range matches {
		sectionStart := match[0]
		sectionEnd := len(content)
		if i+1 < len(matches) {
			sectionEnd = matches[i+1][0]
		}
		sectionContent := content[sectionStart:sectionEnd]
		lines := strings.Split(sectionContent, "\n")
		nonEmptyLines := 0
		for _, line := range lines[1:] { // Skip heading line
			if strings.TrimSpace(line) != "" {
				nonEmptyLines++
			}
		}
		if nonEmptyLines < minNonEmptyLines && nonEmptyLines > 0 {
			details.ShortSectionsCount++
		}
	}
	shortDeduction := float64(details.ShortSectionsCount) * shortSectionDeductionPer
	if shortDeduction > shortSectionDeductionMax {
		shortDeduction = shortSectionDeductionMax
	}
	score -= shortDeduction

	if score < 0 {
		score = 0
	}

	return score
}

// countPlaceholders counts standard placeholder markers in content
func countPlaceholders(content string) int {
	standardPlaceholder := "<< INFORMATION NOT AVAILABLE - PLEASE UPDATE >>"
	return strings.Count(content, standardPlaceholder)
}
