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

// representativeKibanaVersions are used to test whether integration and dependency Kibana constraints overlap.
var representativeKibanaVersions = []*semver.Version{
	semver.MustParse("8.0.0"),
	semver.MustParse("8.12.0"),
	semver.MustParse("8.17.0"),
	semver.MustParse("9.0.0"),
	semver.MustParse("9.4.0"),
	semver.MustParse("9.17.0"),
}

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
	candidates, err := kibanaVersionsSatisfyingIntegration(integrationKibana)
	if err != nil {
		return false, err
	}
	depManifest := packages.PackageManifest{
		Conditions: packages.Conditions{
			Kibana: packages.KibanaConditions{Version: dependencyKibana},
		},
	}
	for _, v := range candidates {
		if err := packages.CheckConditions(depManifest, []string{fmt.Sprintf("kibana.version=%s", v.Original())}); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func kibanaVersionsSatisfyingIntegration(integrationKibana string) ([]*semver.Version, error) {
	integrationManifest := packages.PackageManifest{
		Conditions: packages.Conditions{
			Kibana: packages.KibanaConditions{Version: integrationKibana},
		},
	}
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
		if err := packages.CheckConditions(integrationManifest, []string{fmt.Sprintf("kibana.version=%s", key)}); err != nil {
			return
		}
		seen[key] = struct{}{}
		result = append(result, v)
	}
	for _, v := range representativeKibanaVersions {
		add(v)
	}
	for _, branch := range strings.Split(integrationKibana, "||") {
		raw, ok := minVersionFromConstraintBranch(branch)
		if !ok {
			continue
		}
		v, err := semver.NewVersion(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid version in integration kibana constraint: %w", err)
		}
		add(v)
	}
	return result, nil
}

func formatKibanaBumpWarning(depName, depVersion, depKibana, integrationKibana string) string {
	return fmt.Sprintf(
		"package %s %s is available but requires kibana %s; integration conditions.kibana.version is %s — consider bumping conditions.kibana.version",
		depName, depVersion, depKibana, integrationKibana,
	)
}
