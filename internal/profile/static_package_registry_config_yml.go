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
const PackageRegistryConfigFile ConfigFile = "package-registry.config.yml"

// NewPackageRegistryConfig returns a Managed Config
func NewPackageRegistryConfig(_ string, profilePath string) (*SimpleFile, error) {

	return &SimpleFile{
		FileName: string(PackageRegistryConfigFile),
		FilePath: filepath.Join(profilePath, string(PackageRegistryConfigFile)),
		FileBody: packageRegistryConfigYml,
	}, nil
}
