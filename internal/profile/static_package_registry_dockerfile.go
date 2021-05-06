// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"path/filepath"
	"strings"
)

// PackageRegistryBaseImage is the base Docker image of the Elastic Package Registry.
const PackageRegistryBaseImage = "docker.elastic.co/package-registry/distribution:snapshot"

// PackageRegistryDockerfileFile is the dockerfile for the Elastic package registry
const PackageRegistryDockerfileFile ConfigFile = "Dockerfile.package-registry"

const packageRegistryDockerfile = `FROM ` + PackageRegistryBaseImage + `

COPY ${PROFILE_NAME}/package-registry.config.yml /package-registry/config.yml
COPY development/ /packages/development
`

// newPackageRegistryDockerfile returns a new config for the package-registry
func newPackageRegistryDockerfile(profileName string, profilePath string) (*simpleFile, error) {
	newCfg := strings.ReplaceAll(packageRegistryDockerfile, "${PROFILE_NAME}", profileName)

	return &simpleFile{
		name: string(PackageRegistryDockerfileFile),
		path: filepath.Join(profilePath, string(PackageRegistryDockerfileFile)),
		body: newCfg,
	}, nil

}
