// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/logger"
)

// GoldenFileManager manages golden files for documentation comparison
type GoldenFileManager struct {
	// GoldenDir is the directory containing golden files
	GoldenDir string

	// cache holds loaded golden files
	cache map[string]*GoldenFile
}

// GoldenFile represents a known-good documentation example
type GoldenFile struct {
	// PackageName is the name of the package this golden file is for
	PackageName string `json:"package_name"`

	// Content is the golden documentation content
	Content string `json:"content"`

	// Metadata holds additional information about the golden file
	Metadata *GoldenMetadata `json:"metadata,omitempty"`

	// Metrics are pre-computed quality metrics
	Metrics *QualityMetrics `json:"metrics,omitempty"`
}

// GoldenMetadata holds metadata about a golden file
type GoldenMetadata struct {
	// Source indicates where this golden file came from
	Source string `json:"source"`

	// CreatedAt is when the golden file was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the golden file was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// Version is the version of the package when this golden was created
	Version string `json:"version,omitempty"`

	// Notes contains any notes about this golden file
	Notes string `json:"notes,omitempty"`

	// Author is who created/approved this golden file
	Author string `json:"author,omitempty"`

	// QualityScore is the expected quality score
	QualityScore float64 `json:"quality_score,omitempty"`
}

// NewGoldenFileManager creates a new golden file manager
func NewGoldenFileManager(goldenDir string) *GoldenFileManager {
	return &GoldenFileManager{
		GoldenDir: goldenDir,
		cache:     make(map[string]*GoldenFile),
	}
}

// GetGoldenFilePath returns the path to a golden file for a package
func (m *GoldenFileManager) GetGoldenFilePath(packageName string) string {
	return filepath.Join(m.GoldenDir, packageName+".golden.md")
}

// GetGoldenMetaPath returns the path to golden file metadata
func (m *GoldenFileManager) GetGoldenMetaPath(packageName string) string {
	return filepath.Join(m.GoldenDir, packageName+".golden.json")
}

// LoadGoldenFile loads a golden file for a package
func (m *GoldenFileManager) LoadGoldenFile(packageName string) (*GoldenFile, error) {
	// Check cache first
	if golden, ok := m.cache[packageName]; ok {
		return golden, nil
	}

	goldenPath := m.GetGoldenFilePath(packageName)
	metaPath := m.GetGoldenMetaPath(packageName)

	// Read golden content
	content, err := os.ReadFile(goldenPath)
	if err != nil {
		return nil, fmt.Errorf("golden file not found for %s: %w", packageName, err)
	}

	golden := &GoldenFile{
		PackageName: packageName,
		Content:     string(content),
	}

	// Read metadata if it exists
	if metaContent, err := os.ReadFile(metaPath); err == nil {
		var meta GoldenMetadata
		if err := json.Unmarshal(metaContent, &meta); err == nil {
			golden.Metadata = &meta
		}
	}

	// Cache the golden file
	m.cache[packageName] = golden

	return golden, nil
}

// SaveGoldenFile saves a golden file for a package
func (m *GoldenFileManager) SaveGoldenFile(golden *GoldenFile) error {
	if err := os.MkdirAll(m.GoldenDir, 0755); err != nil {
		return fmt.Errorf("failed to create golden directory: %w", err)
	}

	// Save content
	goldenPath := m.GetGoldenFilePath(golden.PackageName)
	if err := os.WriteFile(goldenPath, []byte(golden.Content), 0644); err != nil {
		return fmt.Errorf("failed to write golden file: %w", err)
	}

	// Save metadata
	if golden.Metadata != nil {
		metaPath := m.GetGoldenMetaPath(golden.PackageName)
		metaContent, err := json.MarshalIndent(golden.Metadata, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		if err := os.WriteFile(metaPath, metaContent, 0644); err != nil {
			return fmt.Errorf("failed to write metadata: %w", err)
		}
	}

	// Update cache
	m.cache[golden.PackageName] = golden

	logger.Debugf("Saved golden file for %s", golden.PackageName)

	return nil
}

// HasGoldenFile checks if a golden file exists for a package
func (m *GoldenFileManager) HasGoldenFile(packageName string) bool {
	goldenPath := m.GetGoldenFilePath(packageName)
	_, err := os.Stat(goldenPath)
	return err == nil
}

// ListGoldenFiles returns all available golden files
func (m *GoldenFileManager) ListGoldenFiles() ([]string, error) {
	entries, err := os.ReadDir(m.GoldenDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read golden directory: %w", err)
	}

	var packages []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".golden.md") {
			pkgName := strings.TrimSuffix(name, ".golden.md")
			packages = append(packages, pkgName)
		}
	}

	return packages, nil
}

// DeleteGoldenFile removes a golden file for a package
func (m *GoldenFileManager) DeleteGoldenFile(packageName string) error {
	goldenPath := m.GetGoldenFilePath(packageName)
	metaPath := m.GetGoldenMetaPath(packageName)

	if err := os.Remove(goldenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove golden file: %w", err)
	}

	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		// Metadata is optional, just log
		logger.Debugf("Failed to remove metadata file: %v", err)
	}

	delete(m.cache, packageName)

	return nil
}

// CreateGoldenFromExisting creates a golden file from an existing README
func (m *GoldenFileManager) CreateGoldenFromExisting(packagePath, packageName string, notes string) (*GoldenFile, error) {
	// Read existing README
	readmePath := filepath.Join(packagePath, "_dev", "build", "docs", "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		// Try the docs directory
		readmePath = filepath.Join(packagePath, "docs", "README.md")
		content, err = os.ReadFile(readmePath)
		if err != nil {
			return nil, fmt.Errorf("no README found for package %s", packageName)
		}
	}

	// Load package context for metrics
	pkgCtx, _ := validators.LoadPackageContext(packagePath)

	// Compute metrics
	metrics := ComputeMetrics(string(content), pkgCtx)

	golden := &GoldenFile{
		PackageName: packageName,
		Content:     string(content),
		Metrics:     metrics,
		Metadata: &GoldenMetadata{
			Source:       readmePath,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			Notes:        notes,
			QualityScore: metrics.CompositeScore,
		},
	}

	// Save the golden file
	if err := m.SaveGoldenFile(golden); err != nil {
		return nil, err
	}

	return golden, nil
}

// CompareWithGolden compares generated content against the golden file
func (m *GoldenFileManager) CompareWithGolden(packageName, generatedContent string, pkgCtx *validators.PackageContext) (*GoldenComparisonResult, error) {
	golden, err := m.LoadGoldenFile(packageName)
	if err != nil {
		return nil, err
	}

	result := &GoldenComparisonResult{
		PackageName:   packageName,
		Timestamp:     time.Now(),
		GoldenMetrics: golden.Metrics,
	}

	// Compute metrics for generated content
	result.GeneratedMetrics = ComputeMetrics(generatedContent, pkgCtx)

	// Compute comparison
	comparison := ComputeGoldenComparison(generatedContent, golden.Content, pkgCtx)
	result.Comparison = comparison

	// Determine pass/fail
	result.Passed = result.GeneratedMetrics.CompositeScore >= (golden.Metrics.CompositeScore * 0.9) // Within 90%

	// Calculate improvement
	if golden.Metrics != nil {
		result.ScoreDelta = result.GeneratedMetrics.CompositeScore - golden.Metrics.CompositeScore
		if golden.Metrics.CompositeScore > 0 {
			result.PercentChange = (result.ScoreDelta / golden.Metrics.CompositeScore) * 100
		}
	}

	return result, nil
}

// GoldenComparisonResult holds the result of comparing against a golden file
type GoldenComparisonResult struct {
	PackageName      string            `json:"package_name"`
	Timestamp        time.Time         `json:"timestamp"`
	Passed           bool              `json:"passed"`
	GoldenMetrics    *QualityMetrics   `json:"golden_metrics"`
	GeneratedMetrics *QualityMetrics   `json:"generated_metrics"`
	Comparison       *GoldenComparison `json:"comparison"`
	ScoreDelta       float64           `json:"score_delta"`
	PercentChange    float64           `json:"percent_change"`
}

// GenerateComparisonReport generates a markdown report for the comparison
func (r *GoldenComparisonResult) GenerateComparisonReport() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Golden File Comparison: %s\n\n", r.PackageName))
	sb.WriteString(fmt.Sprintf("**Timestamp**: %s\n\n", r.Timestamp.Format(time.RFC3339)))

	// Pass/Fail status
	status := "✅ PASSED"
	if !r.Passed {
		status = "❌ FAILED"
	}
	sb.WriteString(fmt.Sprintf("**Status**: %s\n\n", status))

	// Score comparison
	sb.WriteString("## Score Comparison\n\n")
	sb.WriteString("| Metric | Golden | Generated | Delta |\n")
	sb.WriteString("|--------|--------|-----------|-------|\n")

	if r.GoldenMetrics != nil && r.GeneratedMetrics != nil {
		sb.WriteString(fmt.Sprintf("| **Composite** | %.1f | %.1f | %+.1f (%.1f%%) |\n",
			r.GoldenMetrics.CompositeScore, r.GeneratedMetrics.CompositeScore,
			r.ScoreDelta, r.PercentChange))
		sb.WriteString(fmt.Sprintf("| Structure | %.1f | %.1f | %+.1f |\n",
			r.GoldenMetrics.StructureScore, r.GeneratedMetrics.StructureScore,
			r.GeneratedMetrics.StructureScore-r.GoldenMetrics.StructureScore))
		sb.WriteString(fmt.Sprintf("| Accuracy | %.1f | %.1f | %+.1f |\n",
			r.GoldenMetrics.AccuracyScore, r.GeneratedMetrics.AccuracyScore,
			r.GeneratedMetrics.AccuracyScore-r.GoldenMetrics.AccuracyScore))
		sb.WriteString(fmt.Sprintf("| Completeness | %.1f | %.1f | %+.1f |\n",
			r.GoldenMetrics.CompletenessScore, r.GeneratedMetrics.CompletenessScore,
			r.GeneratedMetrics.CompletenessScore-r.GoldenMetrics.CompletenessScore))
		sb.WriteString(fmt.Sprintf("| Quality | %.1f | %.1f | %+.1f |\n",
			r.GoldenMetrics.QualityScore, r.GeneratedMetrics.QualityScore,
			r.GeneratedMetrics.QualityScore-r.GoldenMetrics.QualityScore))
		sb.WriteString(fmt.Sprintf("| Placeholders | %d | %d | %+d |\n",
			r.GoldenMetrics.PlaceholderCount, r.GeneratedMetrics.PlaceholderCount,
			r.GeneratedMetrics.PlaceholderCount-r.GoldenMetrics.PlaceholderCount))
	}

	// Section comparison
	if r.Comparison != nil {
		sb.WriteString("\n## Section Coverage\n\n")
		sb.WriteString(fmt.Sprintf("- **Coverage**: %.1f%%\n", r.Comparison.SectionCoverage))
		sb.WriteString(fmt.Sprintf("- **Content Similarity**: %.1f%%\n", r.Comparison.ContentSimilarity))

		if len(r.Comparison.MatchingSections) > 0 {
			sb.WriteString(fmt.Sprintf("- **Matching Sections**: %s\n",
				strings.Join(r.Comparison.MatchingSections, ", ")))
		}

		if len(r.Comparison.MissingSections) > 0 {
			sb.WriteString(fmt.Sprintf("- **Missing Sections**: %s\n",
				strings.Join(r.Comparison.MissingSections, ", ")))
		}

		if len(r.Comparison.ExtraSections) > 0 {
			sb.WriteString(fmt.Sprintf("- **Extra Sections**: %s\n",
				strings.Join(r.Comparison.ExtraSections, ", ")))
		}
	}

	return sb.String()
}

// InitializeDefaultGoldens creates golden files from well-documented packages
func (m *GoldenFileManager) InitializeDefaultGoldens(harness *TestHarness) error {
	// List of known well-documented packages to use as goldens
	wellDocumentedPackages := []string{
		"apache",
		"nginx",
		"system",
		"aws",
	}

	for _, pkgName := range wellDocumentedPackages {
		pkgPath := harness.GetPackagePath(pkgName)
		if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
			logger.Debugf("Package %s not found, skipping golden creation", pkgName)
			continue
		}

		if m.HasGoldenFile(pkgName) {
			logger.Debugf("Golden file already exists for %s, skipping", pkgName)
			continue
		}

		_, err := m.CreateGoldenFromExisting(pkgPath, pkgName,
			"Auto-generated from existing well-documented package")
		if err != nil {
			logger.Debugf("Failed to create golden for %s: %v", pkgName, err)
			continue
		}

		logger.Debugf("Created golden file for %s", pkgName)
	}

	return nil
}

// ValidateGoldenFiles checks that all golden files are valid and have metrics
func (m *GoldenFileManager) ValidateGoldenFiles() ([]string, []string) {
	var valid, invalid []string

	packages, err := m.ListGoldenFiles()
	if err != nil {
		return valid, invalid
	}

	for _, pkgName := range packages {
		golden, err := m.LoadGoldenFile(pkgName)
		if err != nil {
			invalid = append(invalid, fmt.Sprintf("%s: %v", pkgName, err))
			continue
		}

		if golden.Content == "" {
			invalid = append(invalid, fmt.Sprintf("%s: empty content", pkgName))
			continue
		}

		if golden.Metrics == nil {
			invalid = append(invalid, fmt.Sprintf("%s: missing metrics", pkgName))
			continue
		}

		valid = append(valid, pkgName)
	}

	return valid, invalid
}

