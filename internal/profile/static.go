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

// KibanaConfigDefaultFile is the default kibana config file
const KibanaConfigDefaultFile configFile = "kibana.config.default.yml"

//go:embed _static/kibana_config_default.yml
var kibanaConfigDefaultYml string

func newKibanaConfigDefault(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(KibanaConfigDefaultFile),
		path: filepath.Join(profilePath, profileStackPath, string(KibanaConfigDefaultFile)),
		body: kibanaConfigDefaultYml,
	}, nil
}

// KibanaConfig8xFile is the Kibana config file for 8.x stack family
const KibanaConfig8xFile configFile = "kibana.config.8x.yml"

//go:embed _static/kibana_config_8x.yml
var kibanaConfig8xYml string

func newKibanaConfig8x(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(KibanaConfig8xFile),
		path: filepath.Join(profilePath, profileStackPath, string(KibanaConfig8xFile)),
		body: kibanaConfig8xYml,
	}, nil
}

// ElasticsearchConfigDefaultFile is the default Elasticsearch config file
const ElasticsearchConfigDefaultFile configFile = "elasticsearch.config.default.yml"

//go:embed _static/elasticsearch_config_default.yml
var elasticsearchConfigDefaultYml string

func newElasticsearchConfigDefault(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(ElasticsearchConfigDefaultFile),
		path: filepath.Join(profilePath, profileStackPath, string(ElasticsearchConfigDefaultFile)),
		body: elasticsearchConfigDefaultYml,
	}, nil
}

// ElasticsearchConfig8xFile is the Elasticsearch config file for 8.x stack family
const ElasticsearchConfig8xFile configFile = "elasticsearch.config.8x.yml"

//go:embed _static/elasticsearch_config_8x.yml
var elasticsearchConfig8xYml string

func newElasticsearchConfig8x(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(ElasticsearchConfig8xFile),
		path: filepath.Join(profilePath, profileStackPath, string(ElasticsearchConfig8xFile)),
		body: elasticsearchConfig8xYml,
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
