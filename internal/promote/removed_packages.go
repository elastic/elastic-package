// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package promote

import "github.com/elastic/elastic-package/internal/storage"

// DeterminePackagesToBeRemoved method lists packages supposed to be removed from the stage.
func DeterminePackagesToBeRemoved(allPackages storage.PackageVersions, promotedPackages storage.PackageVersions, newestOnly bool) storage.PackageVersions {
	var removed storage.PackageVersions

	for _, p := range allPackages {
		var toBeRemoved bool

		for _, r := range promotedPackages {
			if p.Name != r.Name {
				continue
			}

			if newestOnly {
				toBeRemoved = true
				break
			}

			if p.Equal(r) {
				toBeRemoved = true
			}
		}

		if toBeRemoved {
			removed = append(removed, p)
		}
	}
	return removed
}
