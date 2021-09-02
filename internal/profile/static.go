// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	_ "embed"
	"path/filepath"
	"strings"
)

// SnapshotFile is the docker-compose snapshot.yml file name
const SnapshotFile configFile = "snapshot.yml"

//go:embed _static/docker-compose-stack.yml
var snapshotYml string

// newSnapshotFile returns a Managed Config
func newSnapshotFile(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(SnapshotFile),
		path: filepath.Join(profilePath, profileStackPath, string(SnapshotFile)),
		body: snapshotYml,
	}, nil
}

// KibanaConfigFile is the main kibana config file
const KibanaConfigFile configFile = "kibana.config.yml"

//go:embed _static/kibana_config.yml
var kibanaConfigYml string

// newKibanaConfig returns a Managed Config
func newKibanaConfig(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(KibanaConfigFile),
		path: filepath.Join(profilePath, profileStackPath, string(KibanaConfigFile)),
		body: kibanaConfigYml,
	}, nil
}

// PackageRegistryConfigFile is the config file for the Elastic Package registry
const PackageRegistryConfigFile configFile = "package-registry.config.yml"

//go:embed _static/package_registry.yml
var packageRegistryConfigYml string

// newPackageRegistryConfig returns a Managed Config
func newPackageRegistryConfig(_ string, profilePath string) (*simpleFile, error) {

	return &simpleFile{
		name: string(PackageRegistryConfigFile),
		path: filepath.Join(profilePath, profileStackPath, string(PackageRegistryConfigFile)),
		body: packageRegistryConfigYml,
	}, nil
}

// PackageRegistryBaseImage is the base Docker image of the Elastic Package Registry.
const PackageRegistryBaseImage = "docker.elastic.co/package-registry/distribution:snapshot"

// PackageRegistryDockerfileFile is the dockerfile for the Elastic package registry
const PackageRegistryDockerfileFile configFile = "Dockerfile.package-registry"

//go:embed _static/Dockerfile.package-registry
var packageRegistryDockerfileTmpl string
var packageRegistryDockerfile = strings.Replace(packageRegistryDockerfileTmpl,
	"__BASE_IMAGE__", PackageRegistryBaseImage, -1)

// newPackageRegistryDockerfile returns a new config for the package-registry
func newPackageRegistryDockerfile(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(PackageRegistryDockerfileFile),
		path: filepath.Join(profilePath, profileStackPath, string(PackageRegistryDockerfileFile)),
		body: packageRegistryDockerfile,
	}, nil

}
