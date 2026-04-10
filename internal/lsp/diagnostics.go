// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"io/fs"
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

	// Matches: item [name] is not allowed in folder [path]
	itemInFolderErrorRe = regexp.MustCompile(`^item \[([^\]]+)\] is not allowed in folder \[([^\]]+)\]$`)

	// Matches folder-based errors like:
	// "expecting to find [X] folder in folder [/path/to/dir]"
	// "expecting to find [X] file in folder [/path/to/dir]"
	folderErrorRe = regexp.MustCompile(`in folder \[([^\]]+)\]`)

	// Matches: references found in dashboard kibana/dashboard/foo.json: id (type), ...
	dashboardReferencesErrorRe = regexp.MustCompile(`^references found in dashboard ([^:]+): (.+)$`)

	// Matches: reference found in dashboard kibana/dashboard/foo.json: id (type)
	dashboardReferenceErrorRe = regexp.MustCompile(`^reference found in dashboard ([^:]+): (.+)$`)

	// Matches error code suffix like (SVR00001)
	errorCodeRe = regexp.MustCompile(`\(([A-Z]{2,3}\d{5})\)$`)
)

func validatePackageFS(packageRoot string, fsys fs.FS) map[string][]protocol.Diagnostic {
	return validatePackageWith(packageRoot, fsys, func() (error, error) {
		return validation.ValidateAndFilterFromFS(packageRoot, fsys)
	})
}

func validatePackageWith(packageRoot string, fsys fs.FS, validate func() (error, error)) map[string][]protocol.Diagnostic {
	diagsByFile := make(map[string][]protocol.Diagnostic)

	errs, _ := validate()
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
		for _, expandedErrMsg := range expandDiagnosticMessages(errMsg) {
			filePath, message, code := parseError(expandedErrMsg, packageRoot)

			diag := protocol.Diagnostic{
				Range:    findPositionInFS(packageRoot, filePath, fsys, message),
				Severity: diagnosticSeverityPtr(protocol.DiagnosticSeverityError),
				Source:   strPtr(serverName),
				Message:  message,
			}

			if code != "" {
				diag.Code = &protocol.IntegerOrString{Value: code}
			}

			diagsByFile[filePath] = append(diagsByFile[filePath], diag)
		}
	}

	// Ensure we publish empty diagnostics for files that previously had errors
	// but are now clean. The caller handles this by tracking state.
	// For now, we only publish files that have errors.

	return diagsByFile
}

func expandDiagnosticMessages(errMsg string) []string {
	match := dashboardReferencesErrorRe.FindStringSubmatch(errMsg)
	if match == nil {
		return []string{errMsg}
	}

	refs := strings.Split(match[2], ", ")
	if len(refs) <= 1 {
		return []string{errMsg}
	}

	expanded := make([]string, 0, len(refs))
	for _, ref := range refs {
		expanded = append(expanded, "reference found in dashboard "+match[1]+": "+ref)
	}
	return expanded
}

// parseError extracts the file path, message, and error code from an error string.
func parseError(errMsg string, packageRoot string) (filePath, message, code string) {
	// Try to extract error code.
	if m := errorCodeRe.FindStringSubmatch(errMsg); m != nil {
		code = m[1]
	}

	// Try the "file X is invalid: Y" pattern.
	if m := fileErrorRe.FindStringSubmatch(errMsg); m != nil {
		filePath = resolveErrorPath(m[1], packageRoot)
		message = m[2]
		return filePath, message, code
	}

	// Attribute forbidden items to the exact offending file or directory path.
	if m := itemInFolderErrorRe.FindStringSubmatch(errMsg); m != nil {
		filePath = filepath.Join(resolveErrorPath(m[2], packageRoot), m[1])
		message = errMsg
		return filePath, message, code
	}

	// By-reference dashboard warnings report the file path inline instead of
	// using the standard "file X is invalid" wrapper.
	if m := dashboardReferenceErrorRe.FindStringSubmatch(errMsg); m != nil {
		filePath = resolveErrorPath(m[1], packageRoot)
		message = "reference found in dashboard: " + m[2]
		return filePath, message, code
	}

	if m := dashboardReferencesErrorRe.FindStringSubmatch(errMsg); m != nil {
		filePath = resolveErrorPath(m[1], packageRoot)
		message = "references found in dashboard: " + m[2]
		return filePath, message, code
	}

	// Try folder-based pattern: attribute to manifest.yml in that folder.
	if m := folderErrorRe.FindStringSubmatch(errMsg); m != nil {
		dir := resolveErrorPath(m[1], packageRoot)
		candidate := filepath.Join(dir, "manifest.yml")
		if _, statErr := os.Stat(candidate); statErr == nil {
			filePath = candidate
			message = errMsg
			return filePath, message, code
		}

		filePath = dir
		message = errMsg
		return filePath, message, code
	}

	// Fallback: attribute to manifest.yml.
	filePath = filepath.Join(packageRoot, "manifest.yml")
	message = errMsg
	return filePath, message, code
}

func resolveErrorPath(rawPath, packageRoot string) string {
	if filepath.IsAbs(rawPath) {
		return filepath.Clean(rawPath)
	}

	return filepath.Clean(filepath.Join(packageRoot, rawPath))
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
