// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDocPath(t *testing.T) {
	tests := []struct {
		name          string
		packageRoot   string
		targetDocFile string
		wantErr       bool
		expectedPath  string
	}{
		{
			name:          "valid paths",
			packageRoot:   "/test/package",
			targetDocFile: "README.md",
			wantErr:       false,
			expectedPath:  filepath.Join("/test/package", "_dev", "build", "docs", "README.md"),
		},
		{
			name:          "empty package root",
			packageRoot:   "",
			targetDocFile: "README.md",
			wantErr:       true,
		},
		{
			name:          "empty target doc file",
			packageRoot:   "/test/package",
			targetDocFile: "",
			wantErr:       true,
		},
		{
			name:          "both empty",
			packageRoot:   "",
			targetDocFile: "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DocumentationAgent{
				packageRoot:   tt.packageRoot,
				targetDocFile: tt.targetDocFile,
			}

			got, err := d.getDocPath()
			if (err != nil) != tt.wantErr {
				t.Errorf("getDocPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.expectedPath {
				t.Errorf("getDocPath() = %v, want %v", got, tt.expectedPath)
			}
		})
	}
}

func TestExtractPreservedSections(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "single preserved section",
			content: `Some content
<!-- PRESERVE START -->
This is preserved
<!-- PRESERVE END -->
More content`,
			expected: []string{
				"<!-- PRESERVE START -->\nThis is preserved\n<!-- PRESERVE END -->",
			},
		},
		{
			name: "multiple preserved sections",
			content: `Header
<!-- PRESERVE START -->
First preserved
<!-- PRESERVE END -->
Middle
<!-- PRESERVE START -->
Second preserved
<!-- PRESERVE END -->
Footer`,
			expected: []string{
				"<!-- PRESERVE START -->\nFirst preserved\n<!-- PRESERVE END -->",
				"<!-- PRESERVE START -->\nSecond preserved\n<!-- PRESERVE END -->",
			},
		},
		{
			name:     "no preserved sections",
			content:  "Just regular content",
			expected: []string{},
		},
		{
			name: "unclosed preserved section",
			content: `Content
<!-- PRESERVE START -->
Unclosed section`,
			expected: []string{},
		},
		{
			name:     "empty content",
			content:  "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DocumentationAgent{}
			got := d.extractPreservedSections(tt.content)

			if len(got) != len(tt.expected) {
				t.Errorf("extractPreservedSections() got %d sections, want %d", len(got), len(tt.expected))
				return
			}

			for i, section := range got {
				if section != tt.expected[i] {
					t.Errorf("extractPreservedSections() section %d = %q, want %q", i, section, tt.expected[i])
				}
			}
		})
	}
}

func TestArePreservedSectionsKept(t *testing.T) {
	tests := []struct {
		name            string
		originalContent string
		newContent      string
		expected        bool
	}{
		{
			name: "preserved section kept",
			originalContent: `Content
<!-- PRESERVE START -->
Keep this
<!-- PRESERVE END -->
More content`,
			newContent: `New content
<!-- PRESERVE START -->
Keep this
<!-- PRESERVE END -->
Different footer`,
			expected: true,
		},
		{
			name: "preserved section removed",
			originalContent: `Content
<!-- PRESERVE START -->
Keep this
<!-- PRESERVE END -->
More content`,
			newContent: "New content without preserved section",
			expected:   false,
		},
		{
			name:            "no preserved sections in original",
			originalContent: "Just regular content",
			newContent:      "Completely different content",
			expected:        true, // No sections to preserve
		},
		{
			name: "multiple preserved sections all kept",
			originalContent: `Content
<!-- PRESERVE START -->
First
<!-- PRESERVE END -->
Middle
<!-- PRESERVE START -->
Second
<!-- PRESERVE END -->
End`,
			newContent: `New
<!-- PRESERVE START -->
First
<!-- PRESERVE END -->
Different
<!-- PRESERVE START -->
Second
<!-- PRESERVE END -->
Footer`,
			expected: true,
		},
		{
			name: "one of multiple preserved sections missing",
			originalContent: `Content
<!-- PRESERVE START -->
First
<!-- PRESERVE END -->
Middle
<!-- PRESERVE START -->
Second
<!-- PRESERVE END -->
End`,
			newContent: `New
<!-- PRESERVE START -->
First
<!-- PRESERVE END -->
Footer`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DocumentationAgent{}
			got := d.arePreservedSectionsKept(tt.originalContent, tt.newContent)

			if got != tt.expected {
				t.Errorf("arePreservedSectionsKept() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBackupOriginalReadme(t *testing.T) {
	tests := []struct {
		name           string
		createFile     bool
		fileContent    string
		wantErr        bool
		expectBackedUp bool
	}{
		{
			name:           "backup existing file",
			createFile:     true,
			fileContent:    "Original content",
			wantErr:        false,
			expectBackedUp: true,
		},
		{
			name:           "no existing file",
			createFile:     false,
			wantErr:        false,
			expectBackedUp: false,
		},
		{
			name:           "empty existing file",
			createFile:     true,
			fileContent:    "",
			wantErr:        false,
			expectBackedUp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			docDir := filepath.Join(tmpDir, "_dev", "build", "docs")
			if err := os.MkdirAll(docDir, 0o755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			d := &DocumentationAgent{
				packageRoot:   tmpDir,
				targetDocFile: "README.md",
			}

			docPath := filepath.Join(docDir, "README.md")
			if tt.createFile {
				if err := os.WriteFile(docPath, []byte(tt.fileContent), 0o644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			err := d.backupOriginalReadme()
			if (err != nil) != tt.wantErr {
				t.Errorf("backupOriginalReadme() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.expectBackedUp {
				if d.originalReadmeContent == nil {
					t.Error("Expected content to be backed up, but it was nil")
				} else if *d.originalReadmeContent != tt.fileContent {
					t.Errorf("Backed up content = %q, want %q", *d.originalReadmeContent, tt.fileContent)
				}
			} else {
				if d.originalReadmeContent != nil {
					t.Error("Expected no backup, but content was backed up")
				}
			}
		})
	}
}

func TestRestoreOriginalReadme(t *testing.T) {
	tests := []struct {
		name             string
		originalContent  *string
		currentContent   string
		wantErr          bool
		expectFileExists bool
		expectedContent  string
	}{
		{
			name: "restore existing file",
			originalContent: func() *string {
				s := "Original content"
				return &s
			}(),
			currentContent:   "Modified content",
			wantErr:          false,
			expectFileExists: true,
			expectedContent:  "Original content",
		},
		{
			name:             "remove created file when no original",
			originalContent:  nil,
			currentContent:   "Created content",
			wantErr:          false,
			expectFileExists: false,
		},
		{
			name: "restore empty original",
			originalContent: func() *string {
				s := ""
				return &s
			}(),
			currentContent:   "Modified content",
			wantErr:          false,
			expectFileExists: true,
			expectedContent:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			docDir := filepath.Join(tmpDir, "_dev", "build", "docs")
			if err := os.MkdirAll(docDir, 0o755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			d := &DocumentationAgent{
				packageRoot:           tmpDir,
				targetDocFile:         "README.md",
				originalReadmeContent: tt.originalContent,
			}

			docPath := filepath.Join(docDir, "README.md")
			if err := os.WriteFile(docPath, []byte(tt.currentContent), 0o644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := d.restoreOriginalReadme()
			if (err != nil) != tt.wantErr {
				t.Errorf("restoreOriginalReadme() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check if file exists or not
			content, readErr := os.ReadFile(docPath)
			fileExists := readErr == nil

			if fileExists != tt.expectFileExists {
				t.Errorf("File exists = %v, want %v", fileExists, tt.expectFileExists)
			}

			if tt.expectFileExists && string(content) != tt.expectedContent {
				t.Errorf("Restored content = %q, want %q", string(content), tt.expectedContent)
			}
		})
	}
}

func TestIsReadmeUpdated(t *testing.T) {
	tests := []struct {
		name            string
		originalContent *string
		currentContent  string
		expected        bool
	}{
		{
			name: "content changed",
			originalContent: func() *string {
				s := "Original"
				return &s
			}(),
			currentContent: "Modified",
			expected:       true,
		},
		{
			name: "content unchanged",
			originalContent: func() *string {
				s := "Same content"
				return &s
			}(),
			currentContent: "Same content",
			expected:       false,
		},
		{
			name:            "new file with content",
			originalContent: nil,
			currentContent:  "New content",
			expected:        true,
		},
		{
			name:            "new file empty",
			originalContent: nil,
			currentContent:  "",
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			docDir := filepath.Join(tmpDir, "_dev", "build", "docs")
			if err := os.MkdirAll(docDir, 0o755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			d := &DocumentationAgent{
				packageRoot:           tmpDir,
				targetDocFile:         "README.md",
				originalReadmeContent: tt.originalContent,
			}

			docPath := filepath.Join(docDir, "README.md")
			if err := os.WriteFile(docPath, []byte(tt.currentContent), 0o644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			got, err := d.isReadmeUpdated()
			if err != nil {
				t.Errorf("isReadmeUpdated() error = %v", err)
				return
			}

			if got != tt.expected {
				t.Errorf("isReadmeUpdated() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReadCurrentReadme(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		fileExists  bool
		wantErr     bool
	}{
		{
			name:        "read existing file",
			fileContent: "Test content",
			fileExists:  true,
			wantErr:     false,
		},
		{
			name:        "read empty file",
			fileContent: "",
			fileExists:  true,
			wantErr:     false,
		},
		{
			name:       "file does not exist",
			fileExists: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			docDir := filepath.Join(tmpDir, "_dev", "build", "docs")
			if err := os.MkdirAll(docDir, 0o755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			d := &DocumentationAgent{
				packageRoot:   tmpDir,
				targetDocFile: "README.md",
			}

			docPath := filepath.Join(docDir, "README.md")
			if tt.fileExists {
				if err := os.WriteFile(docPath, []byte(tt.fileContent), 0o644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			got, err := d.readCurrentReadme()
			if (err != nil) != tt.wantErr {
				t.Errorf("readCurrentReadme() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.fileContent {
				t.Errorf("readCurrentReadme() = %q, want %q", got, tt.fileContent)
			}
		})
	}
}

func TestReadServiceInfo(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		createFile  bool
		wantExists  bool
		wantContent string
	}{
		{
			name:        "service info exists",
			fileContent: "# Service Information\nTest content",
			createFile:  true,
			wantExists:  true,
			wantContent: "# Service Information\nTest content",
		},
		{
			name:       "service info does not exist",
			createFile: false,
			wantExists: false,
		},
		{
			name:        "empty service info file",
			fileContent: "",
			createFile:  true,
			wantExists:  true,
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			kbDir := filepath.Join(tmpDir, "docs", "knowledge_base")
			if err := os.MkdirAll(kbDir, 0o755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			d := &DocumentationAgent{
				packageRoot: tmpDir,
			}

			serviceInfoPath := filepath.Join(kbDir, "service_info.md")
			if tt.createFile {
				if err := os.WriteFile(serviceInfoPath, []byte(tt.fileContent), 0o644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			content, exists := d.readServiceInfo()

			if exists != tt.wantExists {
				t.Errorf("readServiceInfo() exists = %v, want %v", exists, tt.wantExists)
			}

			if tt.wantExists && content != tt.wantContent {
				t.Errorf("readServiceInfo() content = %q, want %q", content, tt.wantContent)
			}
		})
	}
}
