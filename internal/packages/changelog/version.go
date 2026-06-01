// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package changelog

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// NextVersionFromRevisions returns the top changelog version incremented by mode.
// mode must be "major", "minor", "patch", or "" (no change). Any other value
// returns an error. Returns 0.0.0 when revisions is empty.
func NextVersionFromRevisions(revisions []Revision, mode string) (*semver.Version, error) {
	var current *semver.Version
	if len(revisions) == 0 {
		current = semver.MustParse("0.0.0")
	} else {
		var err error
		current, err = semver.NewVersion(revisions[0].Version)
		if err != nil {
			return nil, fmt.Errorf("invalid changelog top version %q: %w", revisions[0].Version, err)
		}
	}
	switch mode {
	case "major":
		v := current.IncMajor()
		return &v, nil
	case "minor":
		v := current.IncMinor()
		return &v, nil
	case "patch":
		v := current.IncPatch()
		return &v, nil
	case "":
		return current, nil
	default:
		return nil, fmt.Errorf("invalid next mode %q", mode)
	}
}

// NextVersion reads the changelog top version from packageRoot and returns it
// incremented by mode.
func NextVersion(packageRoot, mode string) (*semver.Version, error) {
	revisions, err := ReadChangelogFromPackageRoot(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("reading changelog failed: %w", err)
	}
	return NextVersionFromRevisions(revisions, mode)
}
