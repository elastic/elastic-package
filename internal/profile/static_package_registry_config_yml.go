// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import "path/filepath"

const packageRegistryConfigYml = `package_paths:
  - /packages/development
  - /packages/production
  - /packages/staging
  - /packages/snapshot
`

// PackageRegistryConfigFile is the config file for the Elastic Package registry
const PackageRegistryConfigFile configFile = "package-registry.config.yml"

// newPackageRegistryConfig returns a Managed Config
func newPackageRegistryConfig(_ string, profilePath string) (*simpleFile, error) {

	return &simpleFile{
		name: string(PackageRegistryConfigFile),
		path: filepath.Join(profilePath, profileStackPath, string(PackageRegistryConfigFile)),
		body: packageRegistryConfigYml,
	}, nil
}
