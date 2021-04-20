// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import "github.com/elastic/elastic-package/internal/packages"

type PackageDescriptor struct {
	Manifest packages.PackageManifest
}

// CreatePackage function bootstraps the new package based on the provided descriptor.
func CreatePackage(packageDescriptor PackageDescriptor) error {
	panic("TODO")
}
