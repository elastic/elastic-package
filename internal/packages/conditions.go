// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

// CheckConditions method compares the given values with conditions in manifest.
// The method is useful to check in advance if the package is compatible with particular stack version.
func CheckConditions(manifest PackageManifest, keyValuePairs []string) error {
	return nil // TODO
}
