// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	stackVersion715  = "7.15.0-SNAPSHOT"
	stackVersion820  = "8.2.0-SNAPSHOT"
	stackVersion8160 = "8.16.0-00000000-SNAPSHOT"

	elasticAgentImageName               = "docker.elastic.co/beats/elastic-agent"
	elasticAgentCompleteLegacyImageName = "docker.elastic.co/beats/elastic-agent-complete"
	elasticAgentCompleteImageName       = "docker.elastic.co/elastic-agent/elastic-agent-complete"
	elasticAgentWolfiImageName          = "docker.elastic.co/elastic-agent/elastic-agent-wolfi"
	elasticsearchImageName              = "docker.elastic.co/elasticsearch/elasticsearch"
	kibanaImageName                     = "docker.elastic.co/kibana/kibana"
	logstashImageName                   = "docker.elastic.co/logstash/logstash"

	applicationConfigurationYmlFile = "config.yml"
)

var (
	elasticAgentCompleteFirstSupportedVersion = semver.MustParse(stackVersion715)
	elasticAgentCompleteOwnNamespaceVersion   = semver.MustParse(stackVersion820)
	elasticAgentWolfiVersion                  = semver.MustParse(stackVersion8160)

	// ProfileNameEnvVar is the name of the environment variable to set the default profile
	ProfileNameEnvVar = environment.WithElasticPackagePrefix("PROFILE")
)

func DefaultConfiguration() *ApplicationConfiguration {
	config := ApplicationConfiguration{}
	config.c.Profile.Current = profile.DefaultProfile

	// Uncomment and use the commented definition of "stack" in case of emergency
	// to define Docker image overrides (stack.image_ref_overrides).
	// The following sample defines overrides for the Elastic stack ver. 7.13.0-SNAPSHOT.
	// It's advised to use latest stable snapshots for the stack snapshot.
	//
	//  config.c.Stack.ImageRefOverrides = map[string]ImageRefs{
	//	"7.13.0-SNAPSHOT": ImageRefs{
	//		ElasticAgent: elasticAgentImageName + `@sha256:76c294cf55654bc28dde72ce936032f34ad5f40c345f3df964924778b249e581`,
	//		Kibana:       kibanaImageName + `@sha256:78ae3b1ca09efee242d2c77597dfab18670e984adb96c2407ec03fe07ceca4f6`,
	//	},
	//  }

	return &config
}

// ApplicationConfiguration represents the configuration of the elastic-package.
type ApplicationConfiguration struct {
	c configFile
}

type configFile struct {
	Stack   stack `yaml:"stack"`
	Profile struct {
		Current string `yaml:"current"`
	} `yaml:"profile"`
}

type stack struct {
	ImageRefOverrides map[string]ImageRefs `yaml:"image_ref_overrides"`
}

func checkImageRefOverride(envVar, fallback string) string {
	refOverride := os.Getenv(envVar)
	return stringOrDefault(refOverride, fallback)
}

func (s stack) ImageRefOverridesForVersion(version string) ImageRefs {
	appConfigImageRefs := s.ImageRefOverrides[version]
	return ImageRefs{
		ElasticAgent:  checkImageRefOverride("ELASTIC_AGENT_IMAGE_REF_OVERRIDE", stringOrDefault(appConfigImageRefs.ElasticAgent, "")),
		Elasticsearch: checkImageRefOverride("ELASTICSEARCH_IMAGE_REF_OVERRIDE", stringOrDefault(appConfigImageRefs.Elasticsearch, "")),
		Kibana:        checkImageRefOverride("KIBANA_IMAGE_REF_OVERRIDE", stringOrDefault(appConfigImageRefs.Kibana, "")),
		Logstash:      checkImageRefOverride("LOGSTASH_IMAGE_REF_OVERRIDE", stringOrDefault(appConfigImageRefs.Logstash, "")),
	}
}

// ImageRefs stores Docker image references used to create the Elastic stack containers.
type ImageRefs struct {
	ElasticAgent  string `yaml:"elastic-agent"`
	Elasticsearch string `yaml:"elasticsearch"`
	Kibana        string `yaml:"kibana"`
	Logstash      string `yaml:"logstash"`
}

// AsEnv method returns key=value representation of image refs.
func (ir ImageRefs) AsEnv() []string {
	var vars []string
	vars = append(vars, "ELASTIC_AGENT_IMAGE_REF="+ir.ElasticAgent)
	vars = append(vars, "ELASTICSEARCH_IMAGE_REF="+ir.Elasticsearch)
	vars = append(vars, "KIBANA_IMAGE_REF="+ir.Kibana)
	vars = append(vars, "LOGSTASH_IMAGE_REF="+ir.Logstash)
	return vars
}

// StackImageRefs function selects the appropriate set of Docker image references for the given stack version.
func (ac *ApplicationConfiguration) StackImageRefs(version string) ImageRefs {
	refs := ac.c.Stack.ImageRefOverridesForVersion(version)
	refs.ElasticAgent = stringOrDefault(refs.ElasticAgent, fmt.Sprintf("%s:%s", selectElasticAgentImageName(version), version))
	refs.Elasticsearch = stringOrDefault(refs.Elasticsearch, fmt.Sprintf("%s:%s", elasticsearchImageName, version))
	refs.Kibana = stringOrDefault(refs.Kibana, fmt.Sprintf("%s:%s", kibanaImageName, version))
	refs.Logstash = stringOrDefault(refs.Logstash, fmt.Sprintf("%s:%s", logstashImageName, version))
	return refs
}

// CurrentProfile returns the current profile, or the default one if not set.
func (ac *ApplicationConfiguration) CurrentProfile() string {
	fromEnv := os.Getenv(ProfileNameEnvVar)
	if fromEnv != "" {
		return fromEnv
	}
	current := ac.c.Profile.Current
	if current == "" {
		return profile.DefaultProfile
	}
	return current
}

// SetCurrentProfile sets the current profile.
func (ac *ApplicationConfiguration) SetCurrentProfile(name string) {
	ac.c.Profile.Current = name
}

// selectElasticAgentImageName function returns the appropriate image name for Elastic-Agent depending on the stack version.
// This is mandatory as "elastic-agent-complete" is available since 7.15.0-SNAPSHOT.
func selectElasticAgentImageName(version string) string {
	if version == "" { // as version is optional and can be empty
		return elasticAgentImageName
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		logger.Errorf("stack version not in semver format (value: %s): %v", v, err)
		return elasticAgentImageName
	}
	if !v.LessThan(elasticAgentWolfiVersion) {
		return elasticAgentWolfiImageName
	}
	if !v.LessThan(elasticAgentCompleteOwnNamespaceVersion) {
		return elasticAgentCompleteImageName
	}
	if !v.LessThan(elasticAgentCompleteFirstSupportedVersion) {
		return elasticAgentCompleteLegacyImageName
	}
	return elasticAgentImageName
}

// Configuration function returns the elastic-package configuration.
func Configuration() (*ApplicationConfiguration, error) {
	configPath, err := locations.NewLocationManager()
	if err != nil {
		return nil, fmt.Errorf("can't read configuration directory: %w", err)
	}

	cfg, err := os.ReadFile(filepath.Join(configPath.RootDir(), applicationConfigurationYmlFile))
	if errors.Is(err, os.ErrNotExist) {
		return DefaultConfiguration(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("can't read configuration file: %w", err)
	}

	var c configFile
	err = yaml.Unmarshal(cfg, &c)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal configuration file: %w", err)
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
