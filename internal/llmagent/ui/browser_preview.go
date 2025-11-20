// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ui

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

//go:embed _static/preview_template.html
var htmlTemplate string

// convertMarkdownToHTML converts markdown content to a complete HTML document with embedded CSS
func convertMarkdownToHTML(markdownContent string) string {
	// Create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(markdownContent))

	// Create HTML renderer with flags
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	// Render markdown to HTML
	htmlBody := markdown.Render(doc, renderer)

	// Use the embedded HTML template and inject the rendered markdown
	return fmt.Sprintf(htmlTemplate, string(htmlBody))
}

// isBrowserAvailable checks if a browser can be opened on the current system
func isBrowserAvailable() bool {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	case "windows":
		cmd = "cmd"
	default:
		return false
	}

	// Check if the command exists
	_, err := exec.LookPath(cmd)
	return err == nil
}

// openInBrowser creates a temporary HTML file and opens it in the default browser
func openInBrowser(htmlContent string) error {
	// Create a temporary file with .html extension
	tmpFile, err := os.CreateTemp("", "elastic-package-docs-*.html")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write HTML content to the file
	if _, err := tmpFile.WriteString(htmlContent); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	tmpFile.Close()

	// Open the file in browser
	if err := openURL(tmpPath); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}

// openURL opens the given URL or file path in the default browser
func openURL(urlOrPath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlOrPath)
	case "linux":
		cmd = exec.Command("xdg-open", urlOrPath)
	case "windows":
		// Windows uses 'start' command which requires cmd.exe
		cmd = exec.Command("cmd", "/c", "start", "", urlOrPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// TryBrowserPreview attempts to display the markdown content in a browser
// Returns true if successful, false if it should fall back to terminal display
func TryBrowserPreview(markdownContent string) bool {
	// Check if browser is available
	if !isBrowserAvailable() {
		return false
	}

	// Convert markdown to HTML
	htmlContent := convertMarkdownToHTML(markdownContent)

	// Open in browser
	if err := openInBrowser(htmlContent); err != nil {
		// If browser opening fails, return false to trigger fallback
		return false
	}

	return true
}
