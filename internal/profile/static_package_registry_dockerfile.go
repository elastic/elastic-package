// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"path/filepath"
)

// PackageRegistryBaseImage is the base Docker image of the Elastic Package Registry.
const PackageRegistryBaseImage = "docker.elastic.co/package-registry/distribution:snapshot"

// PackageRegistryDockerfileFile is the dockerfile for the Elastic package registry
const PackageRegistryDockerfileFile configFile = "Dockerfile.package-registry"

const packageRegistryDockerfile = `FROM ` + PackageRegistryBaseImage + `

ARG PROFILE
COPY profiles/${PROFILE}/stack/package-registry.config.yml /package-registry/config.yml
COPY stack/development/ /packages/development
`

// newPackageRegistryDockerfile returns a new config for the package-registry
func newPackageRegistryDockerfile(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(PackageRegistryDockerfileFile),
		path: filepath.Join(profilePath, profileStackPath, string(PackageRegistryDockerfileFile)),
		body: packageRegistryDockerfile,
	}, nil

}
