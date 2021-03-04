// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"fmt"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
)

// ApplicationConfiguration represents the configuration of the elastic-package.
type ApplicationConfiguration struct {
	c common.MapStr
}

// DefaultStackImageRefs function selects the appropriate set of Docker image references for the default stack version.
func (ac *ApplicationConfiguration) DefaultStackImageRefs() ImageRefs {
	return ac.StackImageRefs(DefaultStackVersion)
}

// StackImageRefs function selects the appropriate set of Docker image references for the given stack version.
func (ac *ApplicationConfiguration) StackImageRefs(version string) ImageRefs {
	stackVersionRef := "stack.imageRefOverrides." + version

	var refs ImageRefs
	elasticAgentRef, _ := ac.c.GetValue(stackVersionRef + ".elastic-agent")
	elasticsearchRef, _ := ac.c.GetValue(stackVersionRef + ".elasticsearch")
	kibanaRef, _ := ac.c.GetValue(stackVersionRef + ".kibana")
	refs.ElasticAgent = stringOrDefault(elasticAgentRef, fmt.Sprintf("%s:%s", elasticAgentImageName, DefaultStackVersion))
	refs.Elasticsearch = stringOrDefault(elasticsearchRef, fmt.Sprintf("%s:%s", elasticsearchImageName, DefaultStackVersion))
	refs.Kibana = stringOrDefault(kibanaRef, fmt.Sprintf("%s:%s", kibanaImageName, DefaultStackVersion))

	return refs
}

// ImageRefs stores Docker image references used to create the Elastic stack containers.
type ImageRefs struct {
	ElasticAgent  string `yaml:"elastic-agent"`
	Elasticsearch string `yaml:"elasticsearch"`
	Kibana        string `yaml:"kibana"`
}

// AsEnv method returns key=value representation of image refs.
func (ir ImageRefs) AsEnv() []string {
	var vars []string
	vars = append(vars, "ELASTIC_AGENT_IMAGE_REF="+ir.ElasticAgent)
	vars = append(vars, "ELASTICSEARCH_IMAGE_REF="+ir.Elasticsearch)
	vars = append(vars, "KIBANA_IMAGE_REF="+ir.Kibana)
	return vars
}

// Configuration function returns the elastic-package configuration.
func Configuration() (*ApplicationConfiguration, error) {
	configPath, err := configurationDir()
	if err != nil {
		return nil, errors.Wrap(err, "can't read configuration directory")
	}

	cfg, err := yaml.NewConfigWithFile(filepath.Join(configPath, applicationConfigurationYmlFile), ucfg.PathSep("."))
	if err != nil {
		return nil, errors.Wrap(err, "can't unmarshal application configuration")
	}

	var c common.MapStr
	err = cfg.Unpack(&c)
	if err != nil {
		return nil, errors.Wrap(err, "can't unpack application configuration")
	}

	return &ApplicationConfiguration{
		c: c,
	}, nil
}

func stringOrDefault(value interface{}, defaultValue string) string {
	return ""
}
