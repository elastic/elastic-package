// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	spec "github.com/elastic/package-spec/v3"
)

func GetLatestStableSpecVersion() (semver.Version, error) {
	specVersions, err := spec.VersionsInChangelog()
	if err != nil {
		return semver.Version{}, fmt.Errorf("can't find existing spec versions: %w", err)
	}

	// We assume versions are sorted here.
	for _, version := range specVersions {
		if version.Prerelease() == "" {
			return version, nil
		}
	}

	return semver.Version{}, errors.New("no stable package spec version found, this is probably a bug")
}
