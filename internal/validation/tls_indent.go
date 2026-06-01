// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// tlsHelperPattern matches {{{tls_cert "..." indent=N}}} and {{{tls_key "..." indent=N}}}.
var tlsHelperPattern = regexp.MustCompile(`\{\{\{tls_(cert|key)\s+"[^"]*"(?:\s+indent=(\d+))?\s*\}\}\}`)

// ValidateTLSHelperIndent checks that tls_cert/tls_key helpers in system test
// configs have an indent= value matching the column where the tag appears.
// A mismatch produces broken YAML at test time because PEM continuation
// lines land at the wrong indentation level.
func ValidateTLSHelperIndent(packageRoot string) error {
	pattern := filepath.Join(packageRoot, "data_stream", "*", "_dev", "test", "system", "test-*-config.yml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing system test configs: %w", err)
	}
	// Also check top-level _dev/test/system/ (for packages without data streams).
	topLevel := filepath.Join(packageRoot, "_dev", "test", "system", "test-*-config.yml")
	topFiles, err := filepath.Glob(topLevel)
	if err != nil {
		return fmt.Errorf("globbing top-level system test configs: %w", err)
	}
	files = append(files, topFiles...)

	var errs []string
	for _, f := range files {
		fileErrs, err := checkTLSIndentInFile(f, packageRoot)
		if err != nil {
			return err
		}
		errs = append(errs, fileErrs...)
	}
	if len(errs) > 0 {
		return fmt.Errorf("TLS helper indent errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func checkTLSIndentInFile(path, packageRoot string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	relPath, err := filepath.Rel(packageRoot, path)
	if err != nil {
		relPath = path
	}

	var errs []string
	for i, line := range strings.Split(string(data), "\n") {
		matches := tlsHelperPattern.FindStringSubmatchIndex(line)
		if matches == nil {
			continue
		}

		col := leadingSpaces(line)
		kind := line[matches[2]:matches[3]] // "cert" or "key"

		// Extract indent=N if present.
		if matches[4] < 0 {
			// No indent parameter — not an error, but the PEM won't
			// be indented for YAML. Skip silently.
			continue
		}
		indentStr := line[matches[4]:matches[5]]
		indent, err := strconv.Atoi(indentStr)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s:%d: tls_%s indent=%s is not a valid integer", relPath, i+1, kind, indentStr))
			continue
		}
		if indent != col {
			errs = append(errs, fmt.Sprintf("%s:%d: tls_%s indent=%d but tag starts at column %d", relPath, i+1, kind, indent, col))
		}
	}
	return errs, nil
}

func leadingSpaces(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}
