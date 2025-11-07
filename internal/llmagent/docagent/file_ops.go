// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	preserveStartMarker = "<!-- PRESERVE START -->"
	preserveEndMarker   = "<!-- PRESERVE END -->"
)

// backupOriginalReadme stores the current documentation file content for potential restoration and comparison to the generated version
func (d *DocumentationAgent) backupOriginalReadme() error {
	docPath, err := d.getDocPath()
	if err != nil {
		return err
	}

	// Check if documentation file exists
	if _, err = os.Stat(docPath); err == nil {
		// Read and store the original content
		if content, err := os.ReadFile(docPath); err == nil {
			contentStr := string(content)
			d.originalReadmeContent = &contentStr
			fmt.Printf("📋 Backed up original %s (%d characters)\n", d.targetDocFile, len(contentStr))
		} else {
			fmt.Printf("⚠️  Could not read original %s for backup: %v\n", d.targetDocFile, err)
			return fmt.Errorf("reading file for backup: %w", err)
		}
	} else {
		d.originalReadmeContent = nil
		fmt.Printf("📋 No existing %s found - will create new one\n", d.targetDocFile)
	}
	return nil
}

// restoreOriginalReadme restores the documentation file to its original state
func (d *DocumentationAgent) restoreOriginalReadme() error {
	docPath, err := d.getDocPath()
	if err != nil {
		return err
	}

	if d.originalReadmeContent != nil {
		// Restore original content
		if err := os.WriteFile(docPath, []byte(*d.originalReadmeContent), 0o644); err != nil {
			fmt.Printf("⚠️  Failed to restore original %s: %v\n", d.targetDocFile, err)
			return fmt.Errorf("restoring original file: %w", err)
		}
		fmt.Printf("🔄 Restored original %s (%d characters)\n", d.targetDocFile, len(*d.originalReadmeContent))
	} else {
		// No original file existed, so remove any file that was created
		if err := os.Remove(docPath); err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("⚠️  Failed to remove created %s: %v\n", d.targetDocFile, err)
				return fmt.Errorf("removing created file: %w", err)
			}
		} else {
			fmt.Printf("🗑️  Removed created %s file - restored to original state (no file)\n", d.targetDocFile)
		}
	}
	return nil
}

// isReadmeUpdated checks if the documentation file has been updated by comparing current content to originalReadmeContent
func (d *DocumentationAgent) isReadmeUpdated() (bool, error) {
	docPath, err := d.getDocPath()
	if err != nil {
		return false, err
	}

	// Read current content
	currentContent, err := os.ReadFile(docPath)
	if err != nil {
		return false, fmt.Errorf("cannot read file: %w", err)
	}

	currentContentStr := string(currentContent)

	// If there was no original content, any new content means it's updated
	if d.originalReadmeContent == nil {
		return currentContentStr != "", nil
	}

	// Compare current content with original content
	return currentContentStr != *d.originalReadmeContent, nil
}

// readCurrentReadme reads the current documentation file content
func (d *DocumentationAgent) readCurrentReadme() (string, error) {
	docPath, err := d.getDocPath()
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(docPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// arePreservedSectionsKept checks if human-edited sections are preserved in the new content
func (d *DocumentationAgent) arePreservedSectionsKept(originalContent, newContent string) bool {
	// Extract preserved sections from original content
	preservedSections := d.extractPreservedSections(originalContent)

	// Check if each preserved section exists in the new content
	for _, content := range preservedSections {
		if !strings.Contains(newContent, content) {
			return false
		}
	}

	return true
}

// extractPreservedSections extracts all human-edited sections from content
func (d *DocumentationAgent) extractPreservedSections(content string) []string {
	sections := make([]string, 0)

	startIdx := 0
	sectionNum := 0

	for {
		start := strings.Index(content[startIdx:], preserveStartMarker)
		if start == -1 {
			break
		}
		start += startIdx

		end := strings.Index(content[start:], preserveEndMarker)
		if end == -1 {
			break
		}
		end += start

		// Extract the full section including markers
		sectionContent := content[start : end+len(preserveEndMarker)]
		sections = append(sections, sectionContent)

		startIdx = end + len(preserveEndMarker)
		sectionNum++
	}

	return sections
}

// readServiceInfo reads the service_info.md file if it exists in docs/knowledge_base/
// Returns the content and whether the file exists
func (d *DocumentationAgent) readServiceInfo() (string, bool) {
	serviceInfoPath := filepath.Join(d.packageRoot, "docs", "knowledge_base", "service_info.md")
	content, err := os.ReadFile(serviceInfoPath)
	if err != nil {
		return "", false
	}
	return string(content), true
}

func (d *DocumentationAgent) getDocPath() (string, error) {
	if d.packageRoot == "" {
		return "", fmt.Errorf("packageRoot cannot be empty")
	}
	if d.targetDocFile == "" {
		return "", fmt.Errorf("targetDocFile cannot be empty")
	}
	return filepath.Join(d.packageRoot, "_dev", "build", "docs", d.targetDocFile), nil
}
