// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// configurationVariantMap is a map of version constraints and their matching configuration variant.
// This map is used to deploy different versions of the Elastic stack with matching configurations.
var configurationVariantMap = map[string]string{
	"8.0-0 - 8.1.x-x": "80",
	"8.2-0 - 8.6.x-x": "86",
	"^8.7-0":          "8x",
}

// stackVariantAsEnv function returns a stack variant based on the given stack version.
// We identified three variants:
// * default, covers all of 7.x branches
// * 80, covers stack versions 8.0.0 to 8.1.x
// * 86, covers stack versions 8.2.0 to 8.6.x, they don't support experimental toggles in Fleet
// * 8x, supports different configuration options in Kibana, covers stack versions 8.2.0+
func stackVariantAsEnv(version string) string {
	return fmt.Sprintf("STACK_VERSION_VARIANT=%s", selectStackVersion(version))
}

func selectStackVersion(version string) string {
	if v, err := semver.NewVersion(version); err == nil {
		for constraint, variant := range configurationVariantMap {
			if checkVersion(v, constraint) {
				return variant
			}
		}
	}
	return "default"
}

func checkVersion(v *semver.Version, constraint string) bool {
	if constraint, err := semver.NewConstraint(constraint); err == nil {
		return constraint.Check(v)
	}
	return false
}
