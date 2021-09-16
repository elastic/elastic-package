// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	elasticPackageEnvPrefix = "ELASTIC_PACKAGE_"
)

// Environment variables describing the stack.
var (
	ElasticsearchHostEnv     = elasticPackageEnvPrefix + "ELASTICSEARCH_HOST"
	ElasticsearchUsernameEnv = elasticPackageEnvPrefix + "ELASTICSEARCH_USERNAME"
	ElasticsearchPasswordEnv = elasticPackageEnvPrefix + "ELASTICSEARCH_PASSWORD"
	KibanaHostEnv            = elasticPackageEnvPrefix + "KIBANA_HOST"
)

var shellInitFormat = "export " + ElasticsearchHostEnv + "=%s\nexport " + ElasticsearchUsernameEnv + "=%s\nexport " +
	ElasticsearchPasswordEnv + "=%s\nexport " + KibanaHostEnv + "=%s"

type kibanaConfiguration struct {
	ElasticsearchUsername string `yaml:"elasticsearch.username"`
	ElasticsearchPassword string `yaml:"elasticsearch.password"`
}

// ShellInit method exposes environment variables that can be used for testing purposes.
func ShellInit(elasticStackProfile *profile.Profile) (string, error) {
	// Read Elasticsearch username and password from Kibana configuration file.
	body, err := os.ReadFile(elasticStackProfile.FetchPath(profile.KibanaConfigFile))
	if err != nil {
		return "", errors.Wrap(err, "error reading Kibana config file")
	}

	var kibanaCfg kibanaConfiguration
	err = yaml.Unmarshal(body, &kibanaCfg)
	if err != nil {
		return "", errors.Wrap(err, "unmarshalling Kibana configuration failed")
	}

	// Read Elasticsearch and Kibana hostnames from Elastic Stack Docker Compose configuration file.
	p, err := compose.NewProject(DockerComposeProjectName, elasticStackProfile.FetchPath(profile.SnapshotFile))
	if err != nil {
		return "", errors.Wrap(err, "could not create docker compose project")
	}

	appConfig, err := install.Configuration()
	if err != nil {
		return "", errors.Wrap(err, "can't read application configuration")
	}

	serviceComposeConfig, err := p.Config(compose.CommandOptions{
		Env: append(appConfig.StackImageRefs(install.DefaultStackVersion).AsEnv(), elasticStackProfile.ComposeEnvVars()...),
	})
	if err != nil {
		return "", errors.Wrap(err, "could not get Docker Compose configuration for service")
	}

	kib := serviceComposeConfig.Services["kibana"]
	kibHostPort := fmt.Sprintf("http://%s:%d", kib.Ports[0].ExternalIP, kib.Ports[0].ExternalPort)

	es := serviceComposeConfig.Services["elasticsearch"]
	esHostPort := fmt.Sprintf("http://%s:%d", es.Ports[0].ExternalIP, es.Ports[0].ExternalPort)

	return fmt.Sprintf(shellInitFormat,
		esHostPort,
		kibanaCfg.ElasticsearchUsername,
		kibanaCfg.ElasticsearchPassword,
		kibHostPort), nil
}
