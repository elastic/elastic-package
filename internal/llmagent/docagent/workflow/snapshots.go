// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/logger"
)

// SnapshotManager handles saving and loading intermediate document snapshots
// for auditing and debugging the staged validation workflow.
type SnapshotManager struct {
	// BaseDir is the directory where snapshots are saved
	BaseDir string

	// SessionID uniquely identifies this workflow session
	SessionID string

	// PackageName is the name of the package being documented
	PackageName string

	// snapshots tracks all saved snapshots
	snapshots []SnapshotMetadata

	// enabled controls whether snapshots are saved
	enabled bool
}

// SnapshotMetadata holds metadata about a saved snapshot
type SnapshotMetadata struct {
	Timestamp  time.Time                            `json:"timestamp"`
	Stage      string                               `json:"stage"`
	Iteration  int                                  `json:"iteration"`
	FilePath   string                               `json:"file_path"`
	MetaPath   string                               `json:"meta_path"`
	ContentLen int                                  `json:"content_length"`
	Valid      bool                                 `json:"valid"`
	IssueCount int                                  `json:"issue_count"`
	Result     *validators.StagedValidationResult `json:"result,omitempty"`
}

// Snapshot represents a complete snapshot with content and metadata
type Snapshot struct {
	Metadata SnapshotMetadata `json:"metadata"`
	Content  string           `json:"content"`
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager(baseDir, packageName string) *SnapshotManager {
	sessionID := time.Now().Format("20060102_150405")

	return &SnapshotManager{
		BaseDir:     baseDir,
		SessionID:   sessionID,
		PackageName: packageName,
		snapshots:   []SnapshotMetadata{},
		enabled:     true,
	}
}

// Enable enables snapshot saving
func (sm *SnapshotManager) Enable() {
	sm.enabled = true
}

// Disable disables snapshot saving
func (sm *SnapshotManager) Disable() {
	sm.enabled = false
}

// IsEnabled returns whether snapshot saving is enabled
func (sm *SnapshotManager) IsEnabled() bool {
	return sm.enabled
}

// GetSessionDir returns the session-specific snapshot directory
func (sm *SnapshotManager) GetSessionDir() string {
	return filepath.Join(sm.BaseDir, sm.PackageName, sm.SessionID)
}

// SaveSnapshot saves a document snapshot with metadata
func (sm *SnapshotManager) SaveSnapshot(content, stage string, iteration int, result *validators.StagedValidationResult) error {
	if !sm.enabled {
		return nil
	}

	// Create session directory
	sessionDir := sm.GetSessionDir()
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		logger.Debugf("Failed to create snapshot directory: %v", err)
		return err
	}

	// Generate filenames
	baseName := fmt.Sprintf("%03d_%s_iter%d", len(sm.snapshots), stage, iteration)
	contentPath := filepath.Join(sessionDir, baseName+".md")
	metaPath := filepath.Join(sessionDir, baseName+"_meta.json")

	// Create metadata
	meta := SnapshotMetadata{
		Timestamp:  time.Now(),
		Stage:      stage,
		Iteration:  iteration,
		FilePath:   contentPath,
		MetaPath:   metaPath,
		ContentLen: len(content),
	}

	if result != nil {
		meta.Valid = result.Valid
		meta.IssueCount = len(result.Issues)
		meta.Result = result
	}

	// Save content
	if err := os.WriteFile(contentPath, []byte(content), 0644); err != nil {
		logger.Debugf("Failed to write snapshot content: %v", err)
		return err
	}

	// Save metadata
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		logger.Debugf("Failed to marshal snapshot metadata: %v", err)
		return err
	}
	if err := os.WriteFile(metaPath, metaJSON, 0644); err != nil {
		logger.Debugf("Failed to write snapshot metadata: %v", err)
		return err
	}

	sm.snapshots = append(sm.snapshots, meta)
	logger.Debugf("Saved snapshot: %s (%d bytes)", baseName, len(content))

	return nil
}

// SaveFinalSnapshot saves the final document with a summary
func (sm *SnapshotManager) SaveFinalSnapshot(content string, result *StagedWorkflowResult) error {
	if !sm.enabled {
		return nil
	}

	sessionDir := sm.GetSessionDir()

	// Save final content
	finalPath := filepath.Join(sessionDir, "final.md")
	if err := os.WriteFile(finalPath, []byte(content), 0644); err != nil {
		return err
	}

	// Save audit report
	auditPath := filepath.Join(sessionDir, "audit_report.md")
	if err := os.WriteFile(auditPath, []byte(result.GenerateAuditReport()), 0644); err != nil {
		return err
	}

	// Save workflow summary
	summary := WorkflowSummary{
		SessionID:       sm.SessionID,
		PackageName:     sm.PackageName,
		Timestamp:       time.Now(),
		Approved:        result.Approved,
		TotalIterations: result.TotalIterations,
		Snapshots:       sm.snapshots,
		FinalContentLen: len(content),
	}

	summaryPath := filepath.Join(sessionDir, "summary.json")
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(summaryPath, summaryJSON, 0644); err != nil {
		return err
	}

	logger.Debugf("Saved final snapshot and audit report to: %s", sessionDir)

	return nil
}

// LoadSnapshot loads a snapshot by filename
func (sm *SnapshotManager) LoadSnapshot(filename string) (*Snapshot, error) {
	sessionDir := sm.GetSessionDir()

	// Load content
	contentPath := filepath.Join(sessionDir, filename+".md")
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot content: %w", err)
	}

	// Load metadata
	metaPath := filepath.Join(sessionDir, filename+"_meta.json")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		// Metadata is optional
		return &Snapshot{
			Content: string(content),
		}, nil
	}

	var meta SnapshotMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot metadata: %w", err)
	}

	return &Snapshot{
		Metadata: meta,
		Content:  string(content),
	}, nil
}

// ListSnapshots returns all saved snapshot metadata for this session
func (sm *SnapshotManager) ListSnapshots() []SnapshotMetadata {
	return sm.snapshots
}

// GetSnapshotCount returns the number of saved snapshots
func (sm *SnapshotManager) GetSnapshotCount() int {
	return len(sm.snapshots)
}

// WorkflowSummary holds a summary of the entire workflow execution
type WorkflowSummary struct {
	SessionID       string             `json:"session_id"`
	PackageName     string             `json:"package_name"`
	Timestamp       time.Time          `json:"timestamp"`
	Approved        bool               `json:"approved"`
	TotalIterations int                `json:"total_iterations"`
	Snapshots       []SnapshotMetadata `json:"snapshots"`
	FinalContentLen int                `json:"final_content_length"`
}

// LoadWorkflowSummary loads a previously saved workflow summary
func LoadWorkflowSummary(summaryPath string) (*WorkflowSummary, error) {
	content, err := os.ReadFile(summaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow summary: %w", err)
	}

	var summary WorkflowSummary
	if err := json.Unmarshal(content, &summary); err != nil {
		return nil, fmt.Errorf("failed to parse workflow summary: %w", err)
	}

	return &summary, nil
}

// CompareSnapshots computes differences between two snapshots
func CompareSnapshots(before, after *Snapshot) *SnapshotDiff {
	diff := &SnapshotDiff{
		BeforeStage:     before.Metadata.Stage,
		AfterStage:      after.Metadata.Stage,
		ContentChanged:  before.Content != after.Content,
		LengthDelta:     len(after.Content) - len(before.Content),
		ValidityChanged: before.Metadata.Valid != after.Metadata.Valid,
	}

	if before.Metadata.Result != nil && after.Metadata.Result != nil {
		diff.IssuesDelta = len(after.Metadata.Result.Issues) - len(before.Metadata.Result.Issues)
	}

	return diff
}

// SnapshotDiff represents differences between two snapshots
type SnapshotDiff struct {
	BeforeStage     string `json:"before_stage"`
	AfterStage      string `json:"after_stage"`
	ContentChanged  bool   `json:"content_changed"`
	LengthDelta     int    `json:"length_delta"`
	ValidityChanged bool   `json:"validity_changed"`
	IssuesDelta     int    `json:"issues_delta"`
}

// GenerateProgressReport creates a report showing progression through stages
func (sm *SnapshotManager) GenerateProgressReport() string {
	var report string

	report += fmt.Sprintf("# Workflow Progress Report\n\n")
	report += fmt.Sprintf("**Session**: %s\n", sm.SessionID)
	report += fmt.Sprintf("**Package**: %s\n", sm.PackageName)
	report += fmt.Sprintf("**Snapshots**: %d\n\n", len(sm.snapshots))

	report += "## Stage Progression\n\n"
	report += "| # | Stage | Iteration | Valid | Issues | Length |\n"
	report += "|---|-------|-----------|-------|--------|--------|\n"

	for i, snap := range sm.snapshots {
		valid := "⏳"
		if snap.Result != nil {
			if snap.Valid {
				valid = "✅"
			} else {
				valid = "❌"
			}
		}

		report += fmt.Sprintf("| %d | %s | %d | %s | %d | %d |\n",
			i, snap.Stage, snap.Iteration, valid, snap.IssueCount, snap.ContentLen)
	}

	return report
}

