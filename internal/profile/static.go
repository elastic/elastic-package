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

// KibanaConfig86File is the Kibana config file for the 8.x stack family (8.2 to 8.6)
const KibanaConfig86File configFile = "kibana.config.86.yml"

//go:embed _static/kibana_config_86.yml
var kibanaConfig86Yml string

func newKibanaConfig86(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(KibanaConfig86File),
		path: filepath.Join(profilePath, profileStackPath, string(KibanaConfig86File)),
		body: kibanaConfig86Yml,
	}, nil
}

// KibanaConfig80File is the Kibana config file for 8.0 stack family (8.0 to 8.1)
const KibanaConfig80File configFile = "kibana.config.80.yml"

//go:embed _static/kibana_config_80.yml
var kibanaConfig80Yml string

func newKibanaConfig80(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(KibanaConfig80File),
		path: filepath.Join(profilePath, profileStackPath, string(KibanaConfig80File)),
		body: kibanaConfig80Yml,
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

// ElasticsearchConfig8xFile is the Elasticsearch config file for 8.x stack family (8.2 to 8.6)
const ElasticsearchConfig86File configFile = "elasticsearch.config.86.yml"

func newElasticsearchConfig86(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(ElasticsearchConfig86File),
		path: filepath.Join(profilePath, profileStackPath, string(ElasticsearchConfig86File)),
		body: elasticsearchConfig8xYml,
	}, nil
}

// ElasticsearchConfig80File is the Elasticsearch virtual config file name for 8.0 stack family (8.0 to 8.1)
// This file does not exist in the source code, since it's identical to the 8x config file.
const ElasticsearchConfig80File configFile = "elasticsearch.config.80.yml"

func newElasticsearchConfig80(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(ElasticsearchConfig80File),
		path: filepath.Join(profilePath, profileStackPath, string(ElasticsearchConfig80File)),
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
const PackageRegistryBaseImage = "docker.elastic.co/package-registry/package-registry:v1.16.3"

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

// ElasticAgent80EnvFile is the .env for the 8.0 stack.
// This file does not exist in the source code, since it's identical to the 8x env file.
const ElasticAgent80EnvFile configFile = "elastic-agent.80.env"

func newElasticAgent80Env(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(ElasticAgent80EnvFile),
		path: filepath.Join(profilePath, profileStackPath, string(ElasticAgent80EnvFile)),
		body: elasticAgent8xEnv,
	}, nil
}

// ElasticAgent86EnvFile is the .env for the 8.6 stack.
// This file does not exist in the source code, since it's identical to the 8x env file.
const ElasticAgent86EnvFile configFile = "elastic-agent.86.env"

func newElasticAgent86Env(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(ElasticAgent86EnvFile),
		path: filepath.Join(profilePath, profileStackPath, string(ElasticAgent86EnvFile)),
		body: elasticAgent8xEnv,
	}, nil
}

// ElasticAgent8xEnvFile is the .env for the 8x stack.
const ElasticAgent8xEnvFile configFile = "elastic-agent.8x.env"

//go:embed _static/elastic-agent_8x.env
var elasticAgent8xEnv string

func newElasticAgent8xEnv(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(ElasticAgent8xEnvFile),
		path: filepath.Join(profilePath, profileStackPath, string(ElasticAgent8xEnvFile)),
		body: elasticAgent8xEnv,
	}, nil
}

// ElasticAgentDefaultEnvFile is the default .env file.
const ElasticAgentDefaultEnvFile configFile = "elastic-agent.default.env"

//go:embed _static/elastic-agent_default.env
var elasticAgentDefaultEnv string

func newElasticAgentDefaultEnv(_ string, profilePath string) (*simpleFile, error) {
	return &simpleFile{
		name: string(ElasticAgentDefaultEnvFile),
		path: filepath.Join(profilePath, profileStackPath, string(ElasticAgentDefaultEnvFile)),
		body: elasticAgentDefaultEnv,
	}, nil
}
