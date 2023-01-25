// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
)

type InitConfig struct {
	ElasticsearchHostPort string
	ElasticsearchUsername string
	ElasticsearchPassword string
	KibanaHostPort        string
	CACertificatePath     string
}

func StackInitConfig(elasticStackProfile *profile.Profile) (*InitConfig, error) {
	// Read Elasticsearch username and password from Kibana configuration file.
	// FIXME read credentials from correct Kibana config file, not default
	body, err := os.ReadFile(elasticStackProfile.FetchPath(profile.KibanaConfigDefaultFile))
	if err != nil {
		return nil, fmt.Errorf("error reading Kibana config file: %s", err)
	}

	var kibanaCfg struct {
		ElasticsearchUsername string `yaml:"elasticsearch.username"`
		ElasticsearchPassword string `yaml:"elasticsearch.password"`
	}
	err = yaml.Unmarshal(body, &kibanaCfg)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling Kibana configuration failed: %s", err)
	}

	// Read Elasticsearch and Kibana hostnames from Elastic Stack Docker Compose configuration file.
	p, err := compose.NewProject(DockerComposeProjectName, elasticStackProfile.FetchPath(profile.SnapshotFile))
	if err != nil {
		return nil, fmt.Errorf("could not create docker compose project: %s", err)
	}

	appConfig, err := install.Configuration()
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %s", err)
	}

	serviceComposeConfig, err := p.Config(compose.CommandOptions{
		Env: newEnvBuilder().
			withEnvs(appConfig.StackImageRefs(install.DefaultStackVersion).AsEnv()).
			withEnvs(elasticStackProfile.ComposeEnvVars()).
			withEnv(stackVariantAsEnv(install.DefaultStackVersion)).
			build(),
	})
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for service: %s", err)
	}

	kib := serviceComposeConfig.Services["kibana"]
	kibHostPort := fmt.Sprintf("https://%s:%d", kib.Ports[0].ExternalIP, kib.Ports[0].ExternalPort)

	es := serviceComposeConfig.Services["elasticsearch"]
	esHostPort := fmt.Sprintf("https://%s:%d", es.Ports[0].ExternalIP, es.Ports[0].ExternalPort)

	caCert := elasticStackProfile.FetchPath(profile.CACertificateFile)

	return &InitConfig{
		ElasticsearchHostPort: esHostPort,
		ElasticsearchUsername: kibanaCfg.ElasticsearchUsername,
		ElasticsearchPassword: kibanaCfg.ElasticsearchPassword,
		KibanaHostPort:        kibHostPort,
		CACertificatePath:     caCert,
	}, nil
}
