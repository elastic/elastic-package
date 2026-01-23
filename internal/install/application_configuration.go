// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/registry"
)

const (
	stackVersion715 = "7.15.0-SNAPSHOT"
	stackVersion820 = "8.2.0-SNAPSHOT"
	// Not setting here 8.16.0-SNAPSHOT to take also into account prerelease versions
	// like 8.16.0-21bba6f5-SNAPSHOT
	stackVersion8160 = "8.16.0-00000000-SNAPSHOT"

	elasticAgentLegacyImageName         = "docker.elastic.co/beats/elastic-agent"
	elasticAgentImageName               = "docker.elastic.co/elastic-agent/elastic-agent"
	elasticAgentCompleteLegacyImageName = "docker.elastic.co/beats/elastic-agent-complete"
	elasticAgentCompleteImageName       = "docker.elastic.co/elastic-agent/elastic-agent-complete"
	elasticAgentCompleteWolfiImageName  = "docker.elastic.co/elastic-agent/elastic-agent-complete-wolfi"
	elasticAgentWolfiImageName          = "docker.elastic.co/elastic-agent/elastic-agent-wolfi"
	elasticsearchImageName              = "docker.elastic.co/elasticsearch/elasticsearch"
	kibanaImageName                     = "docker.elastic.co/kibana/kibana"
	logstashImageName                   = "docker.elastic.co/logstash/logstash"
	isReadyImageName                    = "tianon/true:multiarch"

	applicationConfigurationYmlFile = "config.yml"
)

var (
	elasticAgentCompleteFirstSupportedVersion = semver.MustParse(stackVersion715)
	elasticAgentCompleteOwnNamespaceVersion   = semver.MustParse(stackVersion820)
	elasticAgentWolfiVersion                  = semver.MustParse(stackVersion8160)

	// ProfileNameEnvVar is the name of the environment variable to set the default profile
	ProfileNameEnvVar = environment.WithElasticPackagePrefix("PROFILE")

	disableElasticAgentWolfiEnvVar = environment.WithElasticPackagePrefix("DISABLE_ELASTIC_AGENT_WOLFI")
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
	c              configFile
	agentBaseImage string
	stackVersion   string
	agentVersion   string
}

type configFile struct {
	Stack   stack `yaml:"stack"`
	Profile struct {
		Current string `yaml:"current"`
	} `yaml:"profile"`
	Status struct {
		PackageRegistry  packageRegistrySettings  `yaml:"package_registry,omitempty"`
		KibanaRepository kibanaRepositorySettings `yaml:"kibana_repository,omitempty"`
	} `yaml:"status,omitempty"`
}

type packageRegistrySettings struct {
	BaseURL string `yaml:"base_url,omitempty"`
}

type kibanaRepositorySettings struct {
	BaseURL string `yaml:"base_url,omitempty"`
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
		IsReady:       checkImageRefOverride("ISREADY_IMAGE_REF_OVERRIDE", stringOrDefault(appConfigImageRefs.IsReady, "")),
	}
}

// ImageRefs stores Docker image references used to create the Elastic stack containers.
type ImageRefs struct {
	ElasticAgent  string `yaml:"elastic-agent"`
	Elasticsearch string `yaml:"elasticsearch"`
	Kibana        string `yaml:"kibana"`
	Logstash      string `yaml:"logstash"`
	IsReady       string `yaml:"is_ready"`
}

// AsEnv method returns key=value representation of image refs.
func (ir ImageRefs) AsEnv() []string {
	var vars []string
	vars = append(vars, "ELASTIC_AGENT_IMAGE_REF="+ir.ElasticAgent)
	vars = append(vars, "ELASTICSEARCH_IMAGE_REF="+ir.Elasticsearch)
	vars = append(vars, "KIBANA_IMAGE_REF="+ir.Kibana)
	vars = append(vars, "LOGSTASH_IMAGE_REF="+ir.Logstash)
	vars = append(vars, "ISREADY_IMAGE_REF="+ir.IsReady)
	return vars
}

// StackImageRefs function selects the appropriate set of Docker image references for the given stack version.
func (ac *ApplicationConfiguration) StackImageRefs() ImageRefs {
	refs := ac.c.Stack.ImageRefOverridesForVersion(ac.stackVersion)
	refs.ElasticAgent = stringOrDefault(refs.ElasticAgent, fmt.Sprintf("%s:%s", selectElasticAgentImageName(ac.agentVersion, ac.agentBaseImage), ac.agentVersion))
	refs.Elasticsearch = stringOrDefault(refs.Elasticsearch, fmt.Sprintf("%s:%s", elasticsearchImageName, ac.stackVersion))
	refs.Kibana = stringOrDefault(refs.Kibana, fmt.Sprintf("%s:%s", kibanaImageName, ac.stackVersion))
	refs.Logstash = stringOrDefault(refs.Logstash, fmt.Sprintf("%s:%s", logstashImageName, ac.stackVersion))
	refs.IsReady = stringOrDefault(refs.IsReady, isReadyImageName)
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

// PackageRegistryBaseURL returns the configured package registry URL,
// falling back to production if not specified
func (ac *ApplicationConfiguration) PackageRegistryBaseURL() string {
	if ac.c.Status.PackageRegistry.BaseURL != "" {
		return ac.c.Status.PackageRegistry.BaseURL
	}
	return registry.ProductionURL
}

// KibanaRepositoryBaseURL returns the configured Kibana repository URL,
// falling back to the default GitHub URL if not specified
func (ac *ApplicationConfiguration) KibanaRepositoryBaseURL() string {
	if ac.c.Status.KibanaRepository.BaseURL != "" {
		return ac.c.Status.KibanaRepository.BaseURL
	}
	return "https://raw.githubusercontent.com/elastic/kibana"
}

// selectElasticAgentImageName function returns the appropriate image name for Elastic-Agent depending on the stack version.
// This is mandatory as "elastic-agent-complete" is available since 7.15.0-SNAPSHOT.
func selectElasticAgentImageName(agentVersion, agentBaseImage string) string {
	if agentVersion == "" { // as version is optional and can be empty
		return elasticAgentWolfiImageName
	}

	v, err := semver.NewVersion(agentVersion)
	if err != nil {
		logger.Errorf("agent version not in semver format (value: %s): %v", agentVersion, err)
		return elasticAgentWolfiImageName
	}

	shouldUseWolfiImage := shouldUseWolfiImages(v)

	switch agentBaseImage {
	case "systemd":
		return selectElasticAgentSystemDImageName(v)
	case "complete":
		if shouldUseWolfiImage {
			return elasticAgentCompleteWolfiImageName
		}
		return selectElasticAgentCompleteImageName(v)
	default:
		if shouldUseWolfiImage {
			return elasticAgentWolfiImageName
		}
		return selectElasticAgentCompleteImageName(v)
	}
}

func shouldUseWolfiImages(version *semver.Version) bool {
	disableWolfiImages := false
	valueEnv, ok := os.LookupEnv(disableElasticAgentWolfiEnvVar)
	if ok && strings.ToLower(valueEnv) != "false" {
		disableWolfiImages = true
	}

	return !disableWolfiImages && !version.LessThan(elasticAgentWolfiVersion)
}

func selectElasticAgentCompleteImageName(version *semver.Version) string {
	switch {
	case !version.LessThan(elasticAgentCompleteOwnNamespaceVersion):
		return elasticAgentCompleteImageName
	case !version.LessThan(elasticAgentCompleteFirstSupportedVersion):
		return elasticAgentCompleteLegacyImageName
	default:
		return elasticAgentLegacyImageName
	}
}

func selectElasticAgentSystemDImageName(version *semver.Version) string {
	if !version.LessThan(elasticAgentCompleteOwnNamespaceVersion) {
		return elasticAgentImageName
	}
	return elasticAgentLegacyImageName
}

type configurationOptions struct {
	agentBaseImage string
	stackVersion   string
	agentVersion   string
}

type ConfigurationOption func(*configurationOptions)

// OptionWithAgentBaseImage sets the agent image type to be used.
func OptionWithAgentBaseImage(agentBaseImage string) ConfigurationOption {
	return func(opts *configurationOptions) {
		opts.agentBaseImage = agentBaseImage
	}
}

// OptionWithStackVersion sets the Elastic Stack version to be used.
func OptionWithStackVersion(stackVersion string) ConfigurationOption {
	return func(opts *configurationOptions) {
		opts.stackVersion = stackVersion
	}
}

// OptionWithAgentVersion sets the Elastic Agent version to be used.
func OptionWithAgentVersion(agentVersion string) ConfigurationOption {
	return func(opts *configurationOptions) {
		opts.agentVersion = agentVersion
	}
}

// Configuration function returns the elastic-package configuration.
func Configuration(options ...ConfigurationOption) (*ApplicationConfiguration, error) {
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

	configOptions := configurationOptions{}
	for _, option := range options {
		option(&configOptions)
	}

	configuration := ApplicationConfiguration{
		c:              c,
		agentBaseImage: configOptions.agentBaseImage,
		stackVersion:   configOptions.stackVersion,
		agentVersion:   configOptions.agentVersion,
	}

	return &configuration, nil
}

func stringOrDefault(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
