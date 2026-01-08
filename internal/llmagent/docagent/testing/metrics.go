// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testing

import (
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
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
	RequiredSectionsFound    int      `json:"required_sections_found"`
	RequiredSectionsTotal    int      `json:"required_sections_total"`
	MissingSections          []string `json:"missing_sections,omitempty"`
	HeadingHierarchyValid    bool     `json:"heading_hierarchy_valid"`

	// Accuracy details
	PackageNameFound         bool     `json:"package_name_found"`
	PackageTitleFound        bool     `json:"package_title_found"`
	DataStreamsDocumented    int      `json:"data_streams_documented"`
	DataStreamsTotal         int      `json:"data_streams_total"`
	MissingDataStreams       []string `json:"missing_data_streams,omitempty"`
	FieldReferencesValid     int      `json:"field_references_valid"`
	FieldReferencesInvalid   int      `json:"field_references_invalid"`

	// Completeness details
	HasSetupSection          bool `json:"has_setup_section"`
	HasVendorSetup           bool `json:"has_vendor_setup"`
	HasKibanaSetup           bool `json:"has_kibana_setup"`
	HasValidationSteps       bool `json:"has_validation_steps"`
	HasTroubleshooting       bool `json:"has_troubleshooting"`
	HasReferenceSection      bool `json:"has_reference_section"`

	// Quality details
	TodoMarkersFound         int  `json:"todo_markers_found"`
	VaguePhrasesFound        int  `json:"vague_phrases_found"`
	PassiveVoiceInstances    int  `json:"passive_voice_instances"`
	ShortSectionsCount       int  `json:"short_sections_count"`

	// Content stats
	TotalWordCount           int `json:"total_word_count"`
	TotalLineCount           int `json:"total_line_count"`
	CodeBlockCount           int `json:"code_block_count"`
}

// Weight constants for composite score calculation
const (
	WeightStructure    = 0.20
	WeightAccuracy     = 0.30
	WeightCompleteness = 0.25
	WeightQuality      = 0.15
	WeightPlaceholders = 0.10
)

// Required sections for structure scoring
var requiredSections = []string{
	"overview",
	"compatibility",
	"setup",
}

// Recommended sections
var recommendedSections = []string{
	"data streams",
	"reference",
	"troubleshooting",
}

// ComputeMetrics calculates quality metrics for documentation content
func ComputeMetrics(content string, pkgCtx *validators.PackageContext) *QualityMetrics {
	metrics := &QualityMetrics{
		Details: &MetricsDetails{},
	}

	contentLower := strings.ToLower(content)

	// Compute individual scores
	metrics.StructureScore = computeStructureScore(content, contentLower, metrics.Details)
	metrics.AccuracyScore = computeAccuracyScore(content, contentLower, pkgCtx, metrics.Details)
	metrics.CompletenessScore = computeCompletenessScore(content, contentLower, pkgCtx, metrics.Details)
	metrics.QualityScore = computeQualityScore(content, contentLower, metrics.Details)
	metrics.PlaceholderCount = countPlaceholders(content)

	// Compute composite score
	placeholderPenalty := float64(metrics.PlaceholderCount) * 5
	if placeholderPenalty > 100 {
		placeholderPenalty = 100
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
func computeStructureScore(content, contentLower string, details *MetricsDetails) float64 {
	score := 0.0

	// Check required sections (60 points)
	details.RequiredSectionsTotal = len(requiredSections)
	for _, section := range requiredSections {
		pattern := `(?m)^##\s+` + section
		if regexp.MustCompile(`(?i)` + pattern).MatchString(content) {
			details.RequiredSectionsFound++
		} else {
			details.MissingSections = append(details.MissingSections, section)
		}
	}
	if details.RequiredSectionsTotal > 0 {
		score += (float64(details.RequiredSectionsFound) / float64(details.RequiredSectionsTotal)) * 60
	}

	// Check recommended sections (20 points)
	recommendedFound := 0
	for _, section := range recommendedSections {
		if strings.Contains(contentLower, "## "+section) ||
			strings.Contains(contentLower, "##"+section) {
			recommendedFound++
		}
	}
	if len(recommendedSections) > 0 {
		score += (float64(recommendedFound) / float64(len(recommendedSections))) * 20
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
	if len(matches) > 0 && len(matches[0][1]) != 1 {
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
func computeCompletenessScore(content, contentLower string, pkgCtx *validators.PackageContext, details *MetricsDetails) float64 {
	score := 0.0

	// Check setup section (25 points)
	details.HasSetupSection = strings.Contains(contentLower, "## setup") ||
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
func computeQualityScore(content, contentLower string, details *MetricsDetails) float64 {
	score := 100.0

	// Check for TODO markers (-20 each, max -40)
	todoPatterns := []string{`\bTODO\b`, `\bFIXME\b`, `\bHACK\b`, `\bTBD\b`}
	for _, pattern := range todoPatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		matches := re.FindAllString(content, -1)
		details.TodoMarkersFound += len(matches)
	}
	todoDeduction := float64(details.TodoMarkersFound) * 20
	if todoDeduction > 40 {
		todoDeduction = 40
	}
	score -= todoDeduction

	// Check for vague phrases (-5 each, max -20)
	vaguePhrases := []string{
		`\bsimply\s+`, `\bjust\s+`, `\beasily\s+`,
		`\bobviously\s+`, `\bclearly\s+`,
	}
	for _, pattern := range vaguePhrases {
		re := regexp.MustCompile(`(?i)` + pattern)
		matches := re.FindAllString(content, -1)
		details.VaguePhrasesFound += len(matches)
	}
	vagueDeduction := float64(details.VaguePhrasesFound) * 5
	if vagueDeduction > 20 {
		vagueDeduction = 20
	}
	score -= vagueDeduction

	// Check for excessive passive voice (-10 if > 10 instances)
	passivePattern := regexp.MustCompile(`(?i)\b(?:is|are|was|were|be|been|being)\s+(?:configured|installed|enabled|set|defined|used|required|needed|supported)\b`)
	details.PassiveVoiceInstances = len(passivePattern.FindAllString(content, -1))
	if details.PassiveVoiceInstances > 10 {
		score -= 10
	}

	// Check for short sections (-5 each, max -20)
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
		if nonEmptyLines < 3 && nonEmptyLines > 0 {
			details.ShortSectionsCount++
		}
	}
	shortDeduction := float64(details.ShortSectionsCount) * 5
	if shortDeduction > 20 {
		shortDeduction = 20
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

// ComputeGoldenComparison compares generated content against a golden file
func ComputeGoldenComparison(generated, golden string, pkgCtx *validators.PackageContext) *GoldenComparison {
	comparison := &GoldenComparison{}

	// Compute metrics for both
	comparison.GeneratedMetrics = ComputeMetrics(generated, pkgCtx)
	comparison.GoldenMetrics = ComputeMetrics(golden, pkgCtx)

	// Compute score delta
	comparison.ScoreDelta = comparison.GeneratedMetrics.CompositeScore - comparison.GoldenMetrics.CompositeScore

	// Compare sections
	generatedSections := extractSections(generated)
	goldenSections := extractSections(golden)

	// Find matching sections
	for section := range goldenSections {
		if _, ok := generatedSections[section]; ok {
			comparison.MatchingSections = append(comparison.MatchingSections, section)
		} else {
			comparison.MissingSections = append(comparison.MissingSections, section)
		}
	}

	// Find extra sections
	for section := range generatedSections {
		if _, ok := goldenSections[section]; !ok {
			comparison.ExtraSections = append(comparison.ExtraSections, section)
		}
	}

	// Compute section coverage
	if len(goldenSections) > 0 {
		comparison.SectionCoverage = float64(len(comparison.MatchingSections)) / float64(len(goldenSections)) * 100
	}

	// Simple content similarity (based on common words)
	generatedWords := extractSignificantWords(generated)
	goldenWords := extractSignificantWords(golden)

	commonWords := 0
	for word := range generatedWords {
		if goldenWords[word] {
			commonWords++
		}
	}

	totalWords := len(generatedWords)
	if len(goldenWords) > totalWords {
		totalWords = len(goldenWords)
	}
	if totalWords > 0 {
		comparison.ContentSimilarity = float64(commonWords) / float64(totalWords) * 100
	}

	return comparison
}

// GoldenComparison holds comparison results against a golden file
type GoldenComparison struct {
	GeneratedMetrics  *QualityMetrics `json:"generated_metrics"`
	GoldenMetrics     *QualityMetrics `json:"golden_metrics"`
	ScoreDelta        float64         `json:"score_delta"`
	SectionCoverage   float64         `json:"section_coverage"`
	ContentSimilarity float64         `json:"content_similarity"`
	MatchingSections  []string        `json:"matching_sections"`
	MissingSections   []string        `json:"missing_sections"`
	ExtraSections     []string        `json:"extra_sections"`
}

// extractSections extracts section names from markdown content
func extractSections(content string) map[string]bool {
	sections := make(map[string]bool)
	sectionPattern := regexp.MustCompile(`(?m)^##\s+(.+)$`)
	matches := sectionPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			section := strings.ToLower(strings.TrimSpace(match[1]))
			sections[section] = true
		}
	}

	return sections
}

// extractSignificantWords extracts significant words for similarity comparison
func extractSignificantWords(content string) map[string]bool {
	words := make(map[string]bool)

	// Remove code blocks
	codeBlockPattern := regexp.MustCompile("(?s)```.*?```")
	content = codeBlockPattern.ReplaceAllString(content, "")

	// Extract words (3+ chars, lowercase)
	wordPattern := regexp.MustCompile(`\b[a-zA-Z]{3,}\b`)
	matches := wordPattern.FindAllString(content, -1)

	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "can": true, "had": true,
		"her": true, "was": true, "one": true, "our": true, "out": true,
		"has": true, "have": true, "been": true, "from": true, "this": true,
		"that": true, "with": true, "they": true, "will": true, "would": true,
	}

	for _, word := range matches {
		lower := strings.ToLower(word)
		if !stopWords[lower] {
			words[lower] = true
		}
	}

	return words
}

