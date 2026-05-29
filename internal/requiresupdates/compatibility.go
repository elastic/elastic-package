// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var constraintVersionRE = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func minVersionFromConstraintBranch(branch string) (string, bool) {
	m := constraintVersionRE.FindStringSubmatch(strings.TrimSpace(branch))
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

// kibanaConstraintIsSubset reports whether every Kibana version satisfying
// integrationKibana also satisfies dependencyKibana.
func kibanaConstraintIsSubset(integrationKibana, dependencyKibana string) (bool, error) {
	if dependencyKibana == "" {
		return true, nil
	}
	if integrationKibana == "" {
		return false, nil
	}
	depConstraint, err := semver.NewConstraint(dependencyKibana)
	if err != nil {
		return false, fmt.Errorf("invalid dependency kibana constraint %q: %w", dependencyKibana, err)
	}
	for _, branch := range strings.Split(integrationKibana, "||") {
		branch = strings.TrimSpace(branch)
		branchConstraint, err := semver.NewConstraint(branch)
		if err != nil {
			return false, fmt.Errorf("invalid integration kibana constraint branch %q: %w", branch, err)
		}
		raw, ok := minVersionFromConstraintBranch(branch)
		if !ok {
			continue
		}
		floor, err := semver.NewVersion(raw)
		if err != nil {
			return false, fmt.Errorf("invalid version in kibana constraint: %w", err)
		}
		patchPlusOne, _ := semver.NewVersion(fmt.Sprintf("%d.%d.%d", floor.Major(), floor.Minor(), floor.Patch()+1))
		ceiling, _ := semver.NewVersion(fmt.Sprintf("%d.99999.0", floor.Major()))
		for _, probe := range []*semver.Version{floor, patchPlusOne, ceiling} {
			if probe == nil || !branchConstraint.Check(probe) {
				continue
			}
			if !depConstraint.Check(probe) {
				return false, nil
			}
		}
	}
	return true, nil
}

func formatKibanaBumpWarning(depName, depVersion, depKibana, integrationKibana string) string {
	return fmt.Sprintf(
		"package %s %s is available but requires kibana %s; integration conditions.kibana.version is %s — consider bumping conditions.kibana.version",
		depName, depVersion, depKibana, integrationKibana,
	)
}
