// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
)

const kibanaVersionRequirement = "kibana.version"

type packageRequirements struct {
	kibana struct {
		version *semver.Version
	}
}

// CheckConditions method compares the given values with conditions in manifest.
// The method is useful to check in advance if the package is compatible with particular stack version.
func CheckConditions(manifest PackageManifest, keyValuePairs []string) error {
	requirements, err := parsePackageRequirements(keyValuePairs)
	if err != nil {
		return errors.Wrap(err, "can't parse given keyValue pairs as package conditions")
	}

	// So far, Kibana is the only supported constraint
	if len(manifest.Conditions.Kibana.Version) > 0 && requirements.kibana.version != nil {
		kibanaConstraint, err := semver.NewConstraint(manifest.Conditions.Kibana.Version)
		if err != nil {
			return errors.Wrap(err, "invalid constraint for Kibana")
		}

		logger.Debugf("Verify Kibana version (constraint: %s, requirement: %s)", manifest.Conditions.Kibana.Version, requirements.kibana.version)
		if ok, errs := kibanaConstraint.Validate(requirements.kibana.version); !ok {
			return errors.Wrap(multierror.Error(errs), "Kibana constraint unsatisfied")
		}
		logger.Debugf("Constraint %s = %s is satisfied", kibanaVersionRequirement, manifest.Conditions.Kibana.Version)
	}
	return nil
}

func parsePackageRequirements(keyValuePairs []string) (*packageRequirements, error) {
	var pr packageRequirements

	for _, keyPair := range keyValuePairs {
		s := strings.SplitN(keyPair, "=", 2)
		if len(s) != 2 {
			return nil, fmt.Errorf("invalid key-value pair: %s", keyPair)
		}

		switch s[0] {
		case kibanaVersionRequirement:
			ver, err := semver.NewVersion(s[1])
			if err != nil {
				return nil, errors.Wrap(err, "can't parse kibana.version as valid semver")
			}

			// Constraint validation doesn't handle prerelease tags. It fails with error:
			// "1.2.3-SNAPSHOT a prerelease version and the constraint is only looking for release versions"
			// In order to use the library's constraint validation, prerelease tags must be skipped.
			// Source code reference: https://github.com/Masterminds/semver/blob/7e314bd12e4aa8ea9742b1e4765f3fe65de38c2d/constraints.go#L89
			withoutPrerelease, err := ver.SetPrerelease("") // clean prerelease tag
			if err != nil {
				return nil, errors.Wrap(err, "can't clean prerelease tag from semver")
			}
			pr.kibana.version = &withoutPrerelease
		default:
			return nil, fmt.Errorf("unknown package requirement: %s", s[0])
		}
	}
	return &pr, nil
}
