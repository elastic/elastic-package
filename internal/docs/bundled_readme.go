// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/elastic/elastic-package/internal/packages"
)

// Composable packages bundle the fields and the input documentation of the packages they
// require at build time, so {{ fields }} and {{ inputDocs }} can only be fully rendered from
// a built package. When linting from the source tree, these regions are rendered as
// bundledSentinel and matched against the committed readme ignoring their content.

const (
	// bundledSentinel stands for the content generated from the bundled input packages
	// when rendering a composable readme without a built package.
	bundledSentinel = "\x00elastic-package-bundled\x00"
	// bundledPlaceholder replaces the bundled regions in the diffs shown to the user.
	bundledPlaceholder = "<content generated from the required input packages>"
)

// matchesBundledRegions matches the existing readme against the segments of a readme
// rendered with a sentinel in place of each bundled region: the existing content must start
// with the first segment, end with the last one and contain the rest in order, with
// anything in between.
func matchesBundledRegions(existing string, segments []string) bool {
	if len(segments) == 1 {
		return existing == segments[0]
	}

	if !strings.HasPrefix(existing, segments[0]) {
		return false
	}
	rest := existing[len(segments[0]):]
	for _, segment := range segments[1 : len(segments)-1] {
		idx := strings.Index(rest, segment)
		if idx < 0 {
			return false
		}
		rest = rest[idx+len(segment):]
	}
	return strings.HasSuffix(rest, segments[len(segments)-1])
}

// collapseBundledRegions replaces, in the existing readme, the lines that the line diff
// against the wanted readme (with bundledPlaceholder in the bundled regions) pairs only with
// a placeholder, so that diffs don't show the content of the bundled regions.
func collapseBundledRegions(existing, want string) string {
	existingLines := difflib.SplitLines(existing)
	wantLines := difflib.SplitLines(want)

	var collapsed []string
	for _, op := range difflib.NewMatcher(existingLines, wantLines).GetOpCodes() {
		switch {
		case op.Tag == 'e':
			collapsed = append(collapsed, existingLines[op.I1:op.I2]...)
		case op.Tag == 'r' && isBundledPlaceholder(wantLines[op.J1:op.J2]):
			collapsed = append(collapsed, wantLines[op.J1:op.J2]...)
		case op.Tag != 'i':
			collapsed = append(collapsed, existingLines[op.I1:op.I2]...)
		}
	}
	// difflib.SplitLines terminates every line, including the last one, with a new line.
	return strings.TrimSuffix(strings.Join(collapsed, ""), "\n")
}

func isBundledPlaceholder(lines []string) bool {
	found := false
	for _, line := range lines {
		switch {
		case strings.Contains(line, bundledPlaceholder):
			found = true
		case strings.TrimSpace(line) != "":
			return false
		}
	}
	return found
}

// hasRequiredInputs reports whether the package at packageRoot is a composable integration,
// i.e. type == "integration" with at least one requires.input entry.
func hasRequiredInputs(packageRoot string) (bool, error) {
	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return false, fmt.Errorf("reading package manifest: %w", err)
	}
	return m.Type == "integration" && m.Requires != nil && len(m.Requires.Input) > 0, nil
}
