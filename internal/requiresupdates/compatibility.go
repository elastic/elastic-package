// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/packages"
)

var constraintVersionRE = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

// deriveEPRKibanaVersions returns concrete Kibana versions to pass to EPR search when filtering by stack compatibility.
func deriveEPRKibanaVersions(integrationKibanaConstraint string) []string {
	if integrationKibanaConstraint == "" {
		return nil
	}
	branches := strings.Split(integrationKibanaConstraint, "||")
	seen := make(map[string]struct{})
	var versions []string
	for _, branch := range branches {
		v, ok := minVersionFromConstraintBranch(branch)
		if !ok {
			continue
		}
		if _, exists := seen[v]; exists {
			continue
		}
		seen[v] = struct{}{}
		versions = append(versions, v)
	}
	return versions
}

func minVersionFromConstraintBranch(branch string) (string, bool) {
	m := constraintVersionRE.FindStringSubmatch(strings.TrimSpace(branch))
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

// kibanaConstraintsOverlap reports whether some Kibana version satisfies both integration and dependency constraints.
func kibanaConstraintsOverlap(integrationKibana, dependencyKibana string) (bool, error) {
	if integrationKibana == "" || dependencyKibana == "" {
		return true, nil
	}
	candidates, err := candidateKibanaVersions(integrationKibana, dependencyKibana)
	if err != nil {
		return false, err
	}
	integManifest := packages.PackageManifest{
		Conditions: packages.Conditions{
			Kibana: packages.KibanaConditions{Version: integrationKibana},
		},
	}
	depManifest := packages.PackageManifest{
		Conditions: packages.Conditions{
			Kibana: packages.KibanaConditions{Version: dependencyKibana},
		},
	}
	for _, v := range candidates {
		vStr := fmt.Sprintf("kibana.version=%s", v.Original())
		if packages.CheckConditions(integManifest, []string{vStr}) == nil &&
			packages.CheckConditions(depManifest, []string{vStr}) == nil {
			return true, nil
		}
	}
	return false, nil
}

// candidateKibanaVersions extracts probe versions from one or more constraint strings.
// For each branch it adds the literal version and patch+1 to cover strict-greater-than
// lower bounds (e.g. >9.5.0 → also probe 9.5.1).
func candidateKibanaVersions(constraints ...string) ([]*semver.Version, error) {
	seen := make(map[string]struct{})
	var result []*semver.Version
	add := func(v *semver.Version) {
		if v == nil {
			return
		}
		key := v.Original()
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, v)
	}
	for _, constraint := range constraints {
		for _, branch := range strings.Split(constraint, "||") {
			raw, ok := minVersionFromConstraintBranch(branch)
			if !ok {
				continue
			}
			v, err := semver.NewVersion(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid version in kibana constraint: %w", err)
			}
			add(v)
			next, _ := semver.NewVersion(fmt.Sprintf("%d.%d.%d", v.Major(), v.Minor(), v.Patch()+1))
			add(next)
		}
	}
	return result, nil
}

func formatKibanaBumpWarning(depName, depVersion, depKibana, integrationKibana string) string {
	return fmt.Sprintf(
		"package %s %s is available but requires kibana %s; integration conditions.kibana.version is %s — consider bumping conditions.kibana.version",
		depName, depVersion, depKibana, integrationKibana,
	)
}
