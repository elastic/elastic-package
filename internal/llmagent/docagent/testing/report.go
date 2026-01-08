// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReportGenerator generates evaluation reports comparing test runs
type ReportGenerator struct {
	OutputDir string
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(outputDir string) *ReportGenerator {
	return &ReportGenerator{
		OutputDir: outputDir,
	}
}

// EvaluationReport is the main report structure
type EvaluationReport struct {
	Title           string                     `json:"title"`
	Timestamp       time.Time                  `json:"timestamp"`
	BaselineRunID   string                     `json:"baseline_run_id,omitempty"`
	StagedRunID     string                     `json:"staged_run_id,omitempty"`
	PackageReports  []*PackageReport           `json:"package_reports"`
	Summary         *ReportSummary             `json:"summary"`
	StageAnalysis   map[string]*StageAnalysis  `json:"stage_analysis,omitempty"`
	IssuesFixed     *IssuesFixedSummary        `json:"issues_fixed,omitempty"`
}

// PackageReport holds comparison results for a single package
type PackageReport struct {
	PackageName        string          `json:"package_name"`
	BaselineScore      float64         `json:"baseline_score"`
	StagedScore        float64         `json:"staged_score"`
	ScoreDelta         float64         `json:"score_delta"`
	PercentImprovement float64         `json:"percent_improvement"`
	BaselineIterations int             `json:"baseline_iterations"`
	StagedIterations   int             `json:"staged_iterations"`
	IterationsDelta    int             `json:"iterations_delta"`
	BaselinePassed     bool            `json:"baseline_passed"`
	StagedPassed       bool            `json:"staged_passed"`
	MetricDeltas       *MetricDeltas   `json:"metric_deltas,omitempty"`
	StageResults       []*StageReport  `json:"stage_results,omitempty"`
}

// MetricDeltas holds changes in individual metrics
type MetricDeltas struct {
	StructureDelta    float64 `json:"structure_delta"`
	AccuracyDelta     float64 `json:"accuracy_delta"`
	CompletenessDelta float64 `json:"completeness_delta"`
	QualityDelta      float64 `json:"quality_delta"`
	PlaceholderDelta  int     `json:"placeholder_delta"`
}

// StageReport holds results for a validation stage
type StageReport struct {
	Stage            string  `json:"stage"`
	BaselineValid    bool    `json:"baseline_valid"`
	StagedValid      bool    `json:"staged_valid"`
	ScoreDelta       int     `json:"score_delta"`
	IssuesFixed      int     `json:"issues_fixed"`
	IssuesIntroduced int     `json:"issues_introduced"`
}

// ReportSummary provides aggregate statistics
type ReportSummary struct {
	TotalPackages      int     `json:"total_packages"`
	ImprovedPackages   int     `json:"improved_packages"`
	DegradedPackages   int     `json:"degraded_packages"`
	UnchangedPackages  int     `json:"unchanged_packages"`
	AverageBaseline    float64 `json:"average_baseline"`
	AverageStaged      float64 `json:"average_staged"`
	AverageImprovement float64 `json:"average_improvement"`
	TotalIterations    int     `json:"total_iterations"`
	IterationsSaved    int     `json:"iterations_saved"`
}

// StageAnalysis provides analysis for each validation stage
type StageAnalysis struct {
	StageName      string  `json:"stage_name"`
	PassRate       float64 `json:"pass_rate"`
	AverageScore   float64 `json:"average_score"`
	IssuesFound    int     `json:"issues_found"`
	IssuesFixed    int     `json:"issues_fixed"`
	MostCommonIssues []string `json:"most_common_issues,omitempty"`
}

// IssuesFixedSummary summarizes issues fixed by validators
type IssuesFixedSummary struct {
	TotalIssuesFixed        int `json:"total_issues_fixed"`
	MissingSectionsAdded    int `json:"missing_sections_added"`
	FieldReferencesFixed    int `json:"field_references_fixed"`
	PlaceholdersReplaced    int `json:"placeholders_replaced"`
	QualityIssuesFixed      int `json:"quality_issues_fixed"`
	StructureIssuesFixed    int `json:"structure_issues_fixed"`
}

// GenerateComparisonReport creates a comparison report between baseline and staged runs
func (g *ReportGenerator) GenerateComparisonReport(
	baselineResults []*TestResult,
	stagedResults []*TestResult,
) (*EvaluationReport, error) {

	// Create map of staged results by package name
	stagedMap := make(map[string]*TestResult)
	for _, result := range stagedResults {
		stagedMap[result.PackageName] = result
	}

	report := &EvaluationReport{
		Title:          "Documentation Generation Evaluation Report",
		Timestamp:      time.Now(),
		PackageReports: make([]*PackageReport, 0),
		StageAnalysis:  make(map[string]*StageAnalysis),
		IssuesFixed:    &IssuesFixedSummary{},
	}

	// Set run IDs if available
	if len(baselineResults) > 0 {
		report.BaselineRunID = baselineResults[0].RunID
	}
	if len(stagedResults) > 0 {
		report.StagedRunID = stagedResults[0].RunID
	}

	// Generate package reports
	var totalBaselineScore, totalStagedScore float64
	var totalBaselineIter, totalStagedIter int

	for _, baseline := range baselineResults {
		staged, ok := stagedMap[baseline.PackageName]
		if !ok {
			continue // No matching staged result
		}

		pkgReport := g.comparePackages(baseline, staged)
		report.PackageReports = append(report.PackageReports, pkgReport)

		if baseline.Metrics != nil {
			totalBaselineScore += baseline.Metrics.CompositeScore
		}
		if staged.Metrics != nil {
			totalStagedScore += staged.Metrics.CompositeScore
		}
		totalBaselineIter += baseline.TotalIterations
		totalStagedIter += staged.TotalIterations
	}

	// Generate summary
	report.Summary = g.generateSummary(report.PackageReports, totalBaselineScore, totalStagedScore, totalBaselineIter, totalStagedIter)

	// Generate stage analysis
	report.StageAnalysis = g.analyzeStages(stagedResults)

	return report, nil
}

// comparePackages compares baseline and staged results for a package
func (g *ReportGenerator) comparePackages(baseline, staged *TestResult) *PackageReport {
	pkgReport := &PackageReport{
		PackageName:        baseline.PackageName,
		BaselineIterations: baseline.TotalIterations,
		StagedIterations:   staged.TotalIterations,
		IterationsDelta:    staged.TotalIterations - baseline.TotalIterations,
		BaselinePassed:     baseline.Approved,
		StagedPassed:       staged.Approved,
	}

	if baseline.Metrics != nil {
		pkgReport.BaselineScore = baseline.Metrics.CompositeScore
	}
	if staged.Metrics != nil {
		pkgReport.StagedScore = staged.Metrics.CompositeScore
	}

	pkgReport.ScoreDelta = pkgReport.StagedScore - pkgReport.BaselineScore
	if pkgReport.BaselineScore > 0 {
		pkgReport.PercentImprovement = (pkgReport.ScoreDelta / pkgReport.BaselineScore) * 100
	}

	// Compare metrics
	if baseline.Metrics != nil && staged.Metrics != nil {
		pkgReport.MetricDeltas = &MetricDeltas{
			StructureDelta:    staged.Metrics.StructureScore - baseline.Metrics.StructureScore,
			AccuracyDelta:     staged.Metrics.AccuracyScore - baseline.Metrics.AccuracyScore,
			CompletenessDelta: staged.Metrics.CompletenessScore - baseline.Metrics.CompletenessScore,
			QualityDelta:      staged.Metrics.QualityScore - baseline.Metrics.QualityScore,
			PlaceholderDelta:  baseline.Metrics.PlaceholderCount - staged.Metrics.PlaceholderCount,
		}
	}

	// Compare stage results
	for stage, stagedStage := range staged.StageResults {
		stageReport := &StageReport{
			Stage:       stage,
			StagedValid: stagedStage.Valid,
		}

		if baselineStage, ok := baseline.StageResults[stage]; ok {
			stageReport.BaselineValid = baselineStage.Valid
			stageReport.ScoreDelta = stagedStage.Score - baselineStage.Score
			stageReport.IssuesFixed = len(baselineStage.Issues) - len(stagedStage.Issues)
			if stageReport.IssuesFixed < 0 {
				stageReport.IssuesIntroduced = -stageReport.IssuesFixed
				stageReport.IssuesFixed = 0
			}
		}

		pkgReport.StageResults = append(pkgReport.StageResults, stageReport)
	}

	return pkgReport
}

// generateSummary creates aggregate statistics
func (g *ReportGenerator) generateSummary(
	reports []*PackageReport,
	totalBaseline, totalStaged float64,
	totalBaselineIter, totalStagedIter int,
) *ReportSummary {
	summary := &ReportSummary{
		TotalPackages:   len(reports),
		TotalIterations: totalStagedIter,
	}

	for _, report := range reports {
		if report.ScoreDelta > 0 {
			summary.ImprovedPackages++
		} else if report.ScoreDelta < 0 {
			summary.DegradedPackages++
		} else {
			summary.UnchangedPackages++
		}
	}

	if summary.TotalPackages > 0 {
		summary.AverageBaseline = totalBaseline / float64(summary.TotalPackages)
		summary.AverageStaged = totalStaged / float64(summary.TotalPackages)
		summary.AverageImprovement = summary.AverageStaged - summary.AverageBaseline
	}

	summary.IterationsSaved = totalBaselineIter - totalStagedIter
	if summary.IterationsSaved < 0 {
		summary.IterationsSaved = 0
	}

	return summary
}

// analyzeStages provides analysis for each validation stage
func (g *ReportGenerator) analyzeStages(results []*TestResult) map[string]*StageAnalysis {
	analysis := make(map[string]*StageAnalysis)
	stageCounts := make(map[string]int)
	stageScores := make(map[string]float64)
	stagePassed := make(map[string]int)
	stageIssues := make(map[string][]string)

	for _, result := range results {
		for stage, stageResult := range result.StageResults {
			stageCounts[stage]++
			stageScores[stage] += float64(stageResult.Score)
			if stageResult.Valid {
				stagePassed[stage]++
			}
			stageIssues[stage] = append(stageIssues[stage], stageResult.Issues...)
		}
	}

	for stage, count := range stageCounts {
		sa := &StageAnalysis{
			StageName:    stage,
			IssuesFound:  len(stageIssues[stage]),
		}

		if count > 0 {
			sa.PassRate = float64(stagePassed[stage]) / float64(count) * 100
			sa.AverageScore = stageScores[stage] / float64(count)
		}

		// Find most common issues
		issueCounts := make(map[string]int)
		for _, issue := range stageIssues[stage] {
			issueCounts[issue]++
		}

		// Sort by count
		type issuePair struct {
			issue string
			count int
		}
		var pairs []issuePair
		for issue, count := range issueCounts {
			pairs = append(pairs, issuePair{issue, count})
		}
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].count > pairs[j].count
		})

		// Take top 5
		for i := 0; i < len(pairs) && i < 5; i++ {
			sa.MostCommonIssues = append(sa.MostCommonIssues, pairs[i].issue)
		}

		analysis[stage] = sa
	}

	return analysis
}

// GenerateMarkdownReport generates a markdown formatted report
func (g *ReportGenerator) GenerateMarkdownReport(report *EvaluationReport) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", report.Title))
	sb.WriteString(fmt.Sprintf("**Generated**: %s\n\n", report.Timestamp.Format(time.RFC3339)))

	if report.BaselineRunID != "" {
		sb.WriteString(fmt.Sprintf("**Baseline Run**: %s\n", report.BaselineRunID))
	}
	if report.StagedRunID != "" {
		sb.WriteString(fmt.Sprintf("**Staged Run**: %s\n", report.StagedRunID))
	}
	sb.WriteString("\n")

	// Summary section
	if report.Summary != nil {
		sb.WriteString("## Summary\n\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|--------|-------|\n")
		sb.WriteString(fmt.Sprintf("| Total Packages | %d |\n", report.Summary.TotalPackages))
		sb.WriteString(fmt.Sprintf("| Improved | %d |\n", report.Summary.ImprovedPackages))
		sb.WriteString(fmt.Sprintf("| Degraded | %d |\n", report.Summary.DegradedPackages))
		sb.WriteString(fmt.Sprintf("| Unchanged | %d |\n", report.Summary.UnchangedPackages))
		sb.WriteString(fmt.Sprintf("| Average Baseline Score | %.1f |\n", report.Summary.AverageBaseline))
		sb.WriteString(fmt.Sprintf("| Average Staged Score | %.1f |\n", report.Summary.AverageStaged))
		sb.WriteString(fmt.Sprintf("| Average Improvement | %+.1f |\n", report.Summary.AverageImprovement))
		sb.WriteString(fmt.Sprintf("| Total Iterations | %d |\n", report.Summary.TotalIterations))
		sb.WriteString("\n")
	}

	// Package comparison table
	sb.WriteString("## Package Results\n\n")
	sb.WriteString("| Package | Baseline | Staged | Delta | Improvement |\n")
	sb.WriteString("|---------|----------|--------|-------|-------------|\n")

	for _, pkg := range report.PackageReports {
		status := "✅"
		if !pkg.StagedPassed {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("| %s %s | %.1f | %.1f | %+.1f | %+.1f%% |\n",
			pkg.PackageName, status,
			pkg.BaselineScore, pkg.StagedScore,
			pkg.ScoreDelta, pkg.PercentImprovement))
	}
	sb.WriteString("\n")

	// Per-package analysis
	sb.WriteString("## Per-Package Analysis\n\n")
	for _, pkg := range report.PackageReports {
		sb.WriteString(fmt.Sprintf("### %s\n\n", pkg.PackageName))

		if pkg.MetricDeltas != nil {
			sb.WriteString("| Metric | Delta |\n")
			sb.WriteString("|--------|-------|\n")
			sb.WriteString(fmt.Sprintf("| Structure | %+.1f |\n", pkg.MetricDeltas.StructureDelta))
			sb.WriteString(fmt.Sprintf("| Accuracy | %+.1f |\n", pkg.MetricDeltas.AccuracyDelta))
			sb.WriteString(fmt.Sprintf("| Completeness | %+.1f |\n", pkg.MetricDeltas.CompletenessDelta))
			sb.WriteString(fmt.Sprintf("| Quality | %+.1f |\n", pkg.MetricDeltas.QualityDelta))
			sb.WriteString(fmt.Sprintf("| Placeholders | %+d |\n", pkg.MetricDeltas.PlaceholderDelta))
			sb.WriteString("\n")
		}

		if len(pkg.StageResults) > 0 {
			sb.WriteString("**Stage Results**:\n")
			for _, stage := range pkg.StageResults {
				status := "✅"
				if !stage.StagedValid {
					status = "❌"
				}
				sb.WriteString(fmt.Sprintf("- %s %s: score delta %+d, issues fixed %d\n",
					stage.Stage, status, stage.ScoreDelta, stage.IssuesFixed))
			}
			sb.WriteString("\n")
		}
	}

	// Stage analysis
	if len(report.StageAnalysis) > 0 {
		sb.WriteString("## Stage Analysis\n\n")
		for stage, analysis := range report.StageAnalysis {
			sb.WriteString(fmt.Sprintf("### %s\n\n", stage))
			sb.WriteString(fmt.Sprintf("- Pass Rate: %.1f%%\n", analysis.PassRate))
			sb.WriteString(fmt.Sprintf("- Average Score: %.1f\n", analysis.AverageScore))
			sb.WriteString(fmt.Sprintf("- Issues Found: %d\n", analysis.IssuesFound))

			if len(analysis.MostCommonIssues) > 0 {
				sb.WriteString("- Most Common Issues:\n")
				for _, issue := range analysis.MostCommonIssues {
					sb.WriteString(fmt.Sprintf("  - %s\n", issue))
				}
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// SaveReport saves the report in multiple formats
func (g *ReportGenerator) SaveReport(report *EvaluationReport, name string) error {
	if err := os.MkdirAll(g.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save JSON
	jsonPath := filepath.Join(g.OutputDir, name+".json")
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report to JSON: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	// Save Markdown
	mdPath := filepath.Join(g.OutputDir, name+".md")
	mdContent := g.GenerateMarkdownReport(report)
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		return fmt.Errorf("failed to write Markdown report: %w", err)
	}

	return nil
}

// GenerateSingleResultReport generates a report for a single test result
func (g *ReportGenerator) GenerateSingleResultReport(result *TestResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Test Result: %s\n\n", result.PackageName))
	sb.WriteString(fmt.Sprintf("**Run ID**: %s\n", result.RunID))
	sb.WriteString(fmt.Sprintf("**Timestamp**: %s\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Duration**: %v\n", result.Duration))
	sb.WriteString(fmt.Sprintf("**Approved**: %v\n", result.Approved))
	sb.WriteString(fmt.Sprintf("**Iterations**: %d\n\n", result.TotalIterations))

	if result.Error != "" {
		sb.WriteString(fmt.Sprintf("**Error**: %s\n\n", result.Error))
	}

	if result.Metrics != nil {
		sb.WriteString("## Quality Metrics\n\n")
		sb.WriteString("| Metric | Score |\n")
		sb.WriteString("|--------|-------|\n")
		sb.WriteString(fmt.Sprintf("| **Composite** | **%.1f** |\n", result.Metrics.CompositeScore))
		sb.WriteString(fmt.Sprintf("| Structure | %.1f |\n", result.Metrics.StructureScore))
		sb.WriteString(fmt.Sprintf("| Accuracy | %.1f |\n", result.Metrics.AccuracyScore))
		sb.WriteString(fmt.Sprintf("| Completeness | %.1f |\n", result.Metrics.CompletenessScore))
		sb.WriteString(fmt.Sprintf("| Quality | %.1f |\n", result.Metrics.QualityScore))
		sb.WriteString(fmt.Sprintf("| Placeholders | %d |\n", result.Metrics.PlaceholderCount))
		sb.WriteString("\n")
	}

	if len(result.StageResults) > 0 {
		sb.WriteString("## Stage Results\n\n")
		for stage, stageResult := range result.StageResults {
			status := "✅"
			if !stageResult.Valid {
				status = "❌"
			}
			sb.WriteString(fmt.Sprintf("### %s %s\n\n", stage, status))
			sb.WriteString(fmt.Sprintf("- Score: %d\n", stageResult.Score))
			sb.WriteString(fmt.Sprintf("- Iterations: %d\n", stageResult.Iterations))
			if len(stageResult.Issues) > 0 {
				sb.WriteString("- Issues:\n")
				for _, issue := range stageResult.Issues {
					sb.WriteString(fmt.Sprintf("  - %s\n", issue))
				}
			}
			sb.WriteString("\n")
		}
	}

	if result.SnapshotDir != "" {
		sb.WriteString(fmt.Sprintf("## Snapshots\n\nSnapshots saved to: `%s`\n", result.SnapshotDir))
	}

	return sb.String()
}

// GenerateBatchReport generates a report for a batch test run
func (g *ReportGenerator) GenerateBatchReport(batch *BatchResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Batch Test Report\n\n"))
	sb.WriteString(fmt.Sprintf("**Run ID**: %s\n", batch.RunID))
	sb.WriteString(fmt.Sprintf("**Timestamp**: %s\n", batch.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Duration**: %v\n\n", batch.Duration))

	if batch.Summary != nil {
		sb.WriteString("## Summary\n\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|--------|-------|\n")
		sb.WriteString(fmt.Sprintf("| Total Packages | %d |\n", batch.Summary.TotalPackages))
		sb.WriteString(fmt.Sprintf("| Passed | %d |\n", batch.Summary.PassedPackages))
		sb.WriteString(fmt.Sprintf("| Failed | %d |\n", batch.Summary.FailedPackages))
		sb.WriteString(fmt.Sprintf("| Average Score | %.1f |\n", batch.Summary.AverageScore))
		sb.WriteString(fmt.Sprintf("| Total Iterations | %d |\n", batch.Summary.TotalIterations))
		sb.WriteString("\n")
	}

	sb.WriteString("## Package Results\n\n")
	sb.WriteString("| Package | Status | Score | Iterations |\n")
	sb.WriteString("|---------|--------|-------|------------|\n")

	for _, result := range batch.Results {
		status := "✅"
		if !result.Approved {
			status = "❌"
		}
		score := 0.0
		if result.Metrics != nil {
			score = result.Metrics.CompositeScore
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %.1f | %d |\n",
			result.PackageName, status, score, result.TotalIterations))
	}

	return sb.String()
}

