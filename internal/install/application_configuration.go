// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
)

var elasticAgentCompleteFirstSupportedVersion = semver.MustParse("7.15.0-SNAPSHOT")

// ApplicationConfiguration represents the configuration of the elastic-package.
type ApplicationConfiguration struct {
	c configFile
}

type configFile struct {
	Stack stack `yaml:"stack"`
}

type stack struct {
	ImageRefOverrides map[string]ImageRefs `yaml:"image_ref_overrides"`
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
	refs.ElasticAgent = stringOrDefault(refs.ElasticAgent, fmt.Sprintf("%s:%s", selectElasticAgentImageName(version), version))
	refs.Elasticsearch = stringOrDefault(refs.Elasticsearch, fmt.Sprintf("%s:%s", elasticsearchImageName, version))
	refs.Kibana = stringOrDefault(refs.Kibana, fmt.Sprintf("%s:%s", kibanaImageName, version))
	return refs
}

// selectElasticAgentImageName function returns the appropriate image name for Elastic-Agent depending on the stack version.
// This is mandatory as "elastic-agent-complete" is available since 7.15.0-SNAPSHOT.
func selectElasticAgentImageName(version string) string {
	v, err := semver.NewVersion(version)
	if err != nil {
		logger.Errorf("stack version not in semver format (value: %s): %v", v, err)
	} else if !v.LessThan(elasticAgentCompleteFirstSupportedVersion) {
		return elasticAgentCompleteImageName
	}
	return elasticAgentImageName
}

// Configuration function returns the elastic-package configuration.
func Configuration() (*ApplicationConfiguration, error) {
	configPath, err := locations.NewLocationManager()
	if err != nil {
		return nil, errors.Wrap(err, "can't read configuration directory")
	}

	cfg, err := os.ReadFile(filepath.Join(configPath.RootDir(), applicationConfigurationYmlFile))
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
