// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// ApplicationConfiguration represents the configuration of the elastic-package.
type ApplicationConfiguration struct {
	c configFile
}

type configFile struct {
	Stack stack `yaml:"stack"`
}

type stack struct {
	ImageRefOverrides map[string]ImageRefs `yaml:"imageRefOverrides"`
}

func (s stack) ImageRefOverridesForVersion(version string) ImageRefs {
	refs, ok := s.ImageRefOverrides[version]
	if !ok {
		return ImageRefs{}
	}
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

// DefaultStackImageRefs function selects the appropriate set of Docker image references for the default stack version.
func (ac *ApplicationConfiguration) DefaultStackImageRefs() ImageRefs {
	return ac.StackImageRefs(DefaultStackVersion)
}

// StackImageRefs function selects the appropriate set of Docker image references for the given stack version.
func (ac *ApplicationConfiguration) StackImageRefs(version string) ImageRefs {
	refs := ac.c.Stack.ImageRefOverridesForVersion(version)
	refs.ElasticAgent = stringOrDefault(refs.ElasticAgent, fmt.Sprintf("%s:%s", elasticAgentImageName, DefaultStackVersion))
	refs.Elasticsearch = stringOrDefault(refs.Elasticsearch, fmt.Sprintf("%s:%s", elasticsearchImageName, DefaultStackVersion))
	refs.Kibana = stringOrDefault(refs.Kibana, fmt.Sprintf("%s:%s", kibanaImageName, DefaultStackVersion))
	return refs
}

// Configuration function returns the elastic-package configuration.
func Configuration() (*ApplicationConfiguration, error) {
	configPath, err := configurationDir()
	if err != nil {
		return nil, errors.Wrap(err, "can't read configuration directory")
	}

	cfg, err := ioutil.ReadFile(filepath.Join(configPath, applicationConfigurationYmlFile))
	if err != nil {
		return nil, errors.Wrap(err, "can't read configuration file")
	}

	var c configFile
	err = yaml.Unmarshal(cfg, &c)
	if err != nil {
		return nil, errors.Wrap(err, "can't unmarshal configuration file")
	}

	return &ApplicationConfiguration{
		c: c,
	}, nil
}

func stringOrDefault(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
