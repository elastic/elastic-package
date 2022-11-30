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

type InitConfig struct {
	ElasticsearchHostPort string
	ElasticsearchUsername string
	ElasticsearchPassword string
	KibanaHostPort        string
	CACertificatePath     string
}

func StackInitConfig(elasticStackProfile *profile.Profile) (*InitConfig, error) {
	// Read Elasticsearch username and password from Kibana configuration file.
	body, err := os.ReadFile(elasticStackProfile.Path(profileStackPath, KibanaConfigFile))
	if err != nil {
		return nil, errors.Wrap(err, "error reading Kibana config file")
	}

	var kibanaCfg struct {
		ElasticsearchUsername string `yaml:"elasticsearch.username"`
		ElasticsearchPassword string `yaml:"elasticsearch.password"`
	}
	err = yaml.Unmarshal(body, &kibanaCfg)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling Kibana configuration failed")
	}

	// Read Elasticsearch and Kibana hostnames from Elastic Stack Docker Compose configuration file.
	p, err := compose.NewProject(DockerComposeProjectName, elasticStackProfile.Path(profileStackPath, SnapshotFile))
	if err != nil {
		return nil, errors.Wrap(err, "could not create docker compose project")
	}

	appConfig, err := install.Configuration()
	if err != nil {
		return nil, errors.Wrap(err, "can't read application configuration")
	}

	serviceComposeConfig, err := p.Config(compose.CommandOptions{
		Env: newEnvBuilder().
			withEnvs(appConfig.StackImageRefs(install.DefaultStackVersion).AsEnv()).
			withEnvs(elasticStackProfile.ComposeEnvVars()).
			withEnv(stackVariantAsEnv(install.DefaultStackVersion)).
			build(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not get Docker Compose configuration for service")
	}

	kib := serviceComposeConfig.Services["kibana"]
	kibHostPort := fmt.Sprintf("https://%s:%d", kib.Ports[0].ExternalIP, kib.Ports[0].ExternalPort)

	es := serviceComposeConfig.Services["elasticsearch"]
	esHostPort := fmt.Sprintf("https://%s:%d", es.Ports[0].ExternalIP, es.Ports[0].ExternalPort)

	caCert := elasticStackProfile.Path(profileStackPath, CACertificateFile)

	return &InitConfig{
		ElasticsearchHostPort: esHostPort,
		ElasticsearchUsername: kibanaCfg.ElasticsearchUsername,
		ElasticsearchPassword: kibanaCfg.ElasticsearchPassword,
		KibanaHostPort:        kibHostPort,
		CACertificatePath:     caCert,
	}, nil
}
