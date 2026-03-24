// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/elastic/package-spec/v3/code/go/pkg/specerrors"
	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/elastic/elastic-package/internal/validation"
)

var (
	// Matches: file "path/to/file" is invalid: message
	fileErrorRe = regexp.MustCompile(`^file "([^"]+)" is invalid: (.+)$`)

	// Matches folder-based errors like:
	// "expecting to find [X] folder in folder [/path/to/dir]"
	// "expecting to find [X] file in folder [/path/to/dir]"
	folderErrorRe = regexp.MustCompile(`in folder \[([^\]]+)\]`)

	// Matches error code suffix like (SVR00001)
	errorCodeRe = regexp.MustCompile(`\(([A-Z]{2,3}\d{5})\)$`)
)

// validatePackage runs validation on the package at the given root path and
// returns diagnostics grouped by absolute file path.
func validatePackage(packageRoot string) map[string][]protocol.Diagnostic {
	diagsByFile := make(map[string][]protocol.Diagnostic)

	errs, _ := validation.ValidateAndFilterFromPath(packageRoot)
	if errs == nil {
		// No errors — return empty map (will clear diagnostics).
		// Include the manifest so we clear any previous diagnostics for it.
		manifestPath := filepath.Join(packageRoot, "manifest.yml")
		diagsByFile[manifestPath] = []protocol.Diagnostic{}
		return diagsByFile
	}

	// Split into individual errors.
	var individualErrors []string
	if ve, ok := errs.(specerrors.ValidationErrors); ok {
		for _, e := range ve {
			individualErrors = append(individualErrors, e.Error())
		}
	} else {
		// Single error or unknown type — split on newlines as fallback.
		for _, line := range strings.Split(errs.Error(), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "found ") {
				// Strip leading numbering like "   1. "
				line = stripNumbering(line)
				if line != "" {
					individualErrors = append(individualErrors, line)
				}
			}
		}
	}

	for _, errMsg := range individualErrors {
		filePath, message, code := parseError(errMsg, packageRoot)

		diag := protocol.Diagnostic{
			Range:    findPosition(filePath, message),
			Severity: diagnosticSeverityPtr(protocol.DiagnosticSeverityError),
			Source:   strPtr(serverName),
			Message:  message,
		}

		if code != "" {
			diag.Code = &protocol.IntegerOrString{Value: code}
		}

		diagsByFile[filePath] = append(diagsByFile[filePath], diag)
	}

	// Ensure we publish empty diagnostics for files that previously had errors
	// but are now clean. The caller handles this by tracking state.
	// For now, we only publish files that have errors.

	return diagsByFile
}

// parseError extracts the file path, message, and error code from an error string.
func parseError(errMsg string, packageRoot string) (filePath, message, code string) {
	// Try to extract error code.
	if m := errorCodeRe.FindStringSubmatch(errMsg); m != nil {
		code = m[1]
	}

	// Try the "file X is invalid: Y" pattern.
	if m := fileErrorRe.FindStringSubmatch(errMsg); m != nil {
		rawPath := m[1]
		message = m[2]

		// The path may be absolute or relative to the package root.
		if filepath.IsAbs(rawPath) {
			filePath = rawPath
		} else {
			filePath = filepath.Join(packageRoot, rawPath)
		}
		return filePath, message, code
	}

	// Try folder-based pattern: attribute to manifest.yml in that folder.
	if m := folderErrorRe.FindStringSubmatch(errMsg); m != nil {
		dir := m[1]
		candidate := filepath.Join(dir, "manifest.yml")
		if _, statErr := os.Stat(candidate); statErr == nil {
			filePath = candidate
			message = errMsg
			return filePath, message, code
		}
	}

	// Fallback: attribute to manifest.yml.
	filePath = filepath.Join(packageRoot, "manifest.yml")
	message = errMsg
	return filePath, message, code
}

// stripNumbering removes leading numbering like "   1. " from a line.
func stripNumbering(s string) string {
	re := regexp.MustCompile(`^\s*\d+\.\s+`)
	return re.ReplaceAllString(s, "")
}

func diagnosticSeverityPtr(s protocol.DiagnosticSeverity) *protocol.DiagnosticSeverity {
	return &s
}

func strPtr(s string) *string {
	return &s
}
