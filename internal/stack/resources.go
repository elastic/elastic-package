// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/profile"
)

//go:embed _static
var static embed.FS

const (
	// SnapshotFile is the docker-compose snapshot.yml file name.
	SnapshotFile = "snapshot.yml"

	// ElasticsearchConfigFile is the elasticsearch config file.
	ElasticsearchConfigFile = "elasticsearch.yml"

	// KibanaConfigFile is the kibana config file.
	KibanaConfigFile = "kibana.yml"

	// LogstashConfigFile is the logstash config file.
	LogstashConfigFile = "logstash.conf"

	// KibanaHealthcheckFile is the kibana healthcheck.
	KibanaHealthcheckFile = "kibana_healthcheck.sh"

	// PackageRegistryConfigFile is the config file for the Elastic Package registry
	PackageRegistryConfigFile = "package-registry.yml"

	// PackageRegistryBaseImage is the base Docker image of the Elastic Package Registry.
	PackageRegistryBaseImage = "docker.elastic.co/package-registry/package-registry:v1.23.0"

	// ElasticAgentEnvFile is the elastic agent environment variables file.
	ElasticAgentEnvFile = "elastic-agent.env"

	ElasticAgentFolder = "elastic-agent"

	CertsFolder = "certs"

	profileStackPath = "stack"

	elasticsearchUsername = "elastic"
	elasticsearchPassword = "changeme"

	configAPMEnabled         = "stack.apm_enabled"
	configGeoIPDir           = "stack.geoip_dir"
	configLogstashEnabled    = "stack.logstash_enabled"
	configSelfMonitorEnabled = "stack.self_monitor_enabled"
)

var (
	templateFuncs = template.FuncMap{
		"semverLessThan": semverLessThan,
		"indent":         indent,
	}
	staticSource   = resource.NewSourceFS(static).WithTemplateFuncs(templateFuncs)
	stackResources = []resource.Resource{
		&resource.File{
			Path:    "Dockerfile.package-registry",
			Content: staticSource.Template("_static/Dockerfile.package-registry.tmpl"),
		},
		&resource.File{
			Path:    SnapshotFile,
			Content: staticSource.Template("_static/docker-compose-stack.yml.tmpl"),
		},
		&resource.File{
			Path:    ElasticsearchConfigFile,
			Content: staticSource.Template("_static/elasticsearch.yml.tmpl"),
		},
		&resource.File{
			Path:    "service_tokens",
			Content: staticSource.File("_static/service_tokens"),
		},
		&resource.File{
			Path:         "ingest-geoip/GeoLite2-ASN.mmdb",
			CreateParent: true,
			Content:      staticSource.File("_static/GeoLite2-ASN.mmdb"),
		},
		&resource.File{
			Path:         "ingest-geoip/GeoLite2-City.mmdb",
			CreateParent: true,
			Content:      staticSource.File("_static/GeoLite2-City.mmdb"),
		},
		&resource.File{
			Path:         "ingest-geoip/GeoLite2-Country.mmdb",
			CreateParent: true,
			Content:      staticSource.File("_static/GeoLite2-Country.mmdb"),
		},
		&resource.File{
			Path:    KibanaConfigFile,
			Content: staticSource.Template("_static/kibana.yml.tmpl"),
		},
		&resource.File{
			Path:    LogstashConfigFile,
			Content: staticSource.Template("_static/logstash.conf.tmpl"),
		},
		&resource.File{
			Path:    KibanaHealthcheckFile,
			Content: staticSource.Template("_static/kibana_healthcheck.sh.tmpl"),
		},
		&resource.File{
			Path:    PackageRegistryConfigFile,
			Content: staticSource.File("_static/package-registry.yml"),
		},
		&resource.File{
			Path:    ElasticAgentEnvFile,
			Content: staticSource.Template("_static/elastic-agent.env.tmpl"),
		},
	}
)

func applyResources(profile *profile.Profile, stackVersion string) error {
	stackDir := filepath.Join(profile.ProfilePath, profileStackPath)

	resourceManager := resource.NewManager()
	resourceManager.AddFacter(resource.StaticFacter{
		"registry_base_image":   PackageRegistryBaseImage,
		"elasticsearch_version": stackVersion,
		"kibana_version":        stackVersion,
		"agent_version":         stackVersion,

		"kibana_host":        "https://kibana:5601",
		"fleet_url":          "https://fleet-server:8220",
		"elasticsearch_host": "https://elasticsearch:9200",

		"username": elasticsearchUsername,
		"password": elasticsearchPassword,

		"apm_enabled":          profile.Config(configAPMEnabled, "false"),
		"geoip_dir":            profile.Config(configGeoIPDir, "./ingest-geoip"),
		"logstash_enabled":     profile.Config(configLogstashEnabled, "false"),
		"self_monitor_enabled": profile.Config(configSelfMonitorEnabled, "false"),
	})

	os.MkdirAll(stackDir, 0755)
	resourceManager.RegisterProvider("file", &resource.FileProvider{
		Prefix: stackDir,
	})
	resources := append([]resource.Resource{}, stackResources...)

	// Keeping certificates in the profile directory for backwards compatibility reasons.
	resourceManager.RegisterProvider(CertsFolder, &resource.FileProvider{
		Prefix: profile.ProfilePath,
	})
	certResources, err := initTLSCertificates(CertsFolder, profile.ProfilePath, tlsServices)
	if err != nil {
		return fmt.Errorf("failed to create TLS files: %w", err)
	}

	// Add client certificates if logstash is enabled
	if profile.Config("stack.logstash_enabled", "false") == "true" {
		if err := addClientCertsToResources(resourceManager, certResources); err != nil {
			return fmt.Errorf("Error adding client certificates: %w", err)
		}
	}

	resources = append(resources, certResources...)
	results, err := resourceManager.Apply(resources)
	if err != nil {
		var errors []string
		for _, result := range results {
			if err := result.Err(); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return fmt.Errorf("%w: %s", err, strings.Join(errors, ", "))
	}

	return nil
}

func addClientCertsToResources(resourceManager *resource.Manager, certResources []resource.Resource) error {
	certPath := filepath.Join(CertsFolder, ElasticAgentFolder, "cert.pem")
	keyPath := filepath.Join(CertsFolder, ElasticAgentFolder, "key.pem")

	var certFile, keyFile string
	var err error
	for _, r := range certResources {
		res, _ := r.(*resource.File)

		if strings.Contains(res.Path, ElasticAgentFolder) {
			var buf bytes.Buffer
			if res.Path == certPath {
				err = res.Content(nil, &buf)
				if err != nil {
					return fmt.Errorf("failed to read client certificate: %w", err)
				}
				// Replace newlines with spaces to create proper indentation in the config
				certFile = buf.String()
				continue
			}
			if res.Path == keyPath {
				err = res.Content(nil, &buf)
				if err != nil {
					return fmt.Errorf("failed to read client key: %w", err)
				}
				// Replace newlines with spaces to create proper indentation in the config
				keyFile = buf.String()
				continue
			}
		}
	}

	resourceManager.AddFacter(resource.StaticFacter{
		"logstash_ssl_certificate": certFile,
		"logstash_ssl_key":         keyFile,
	})
	return nil
}

func semverLessThan(a, b string) (bool, error) {
	sa, err := semver.NewVersion(a)
	if err != nil {
		return false, err
	}
	sb, err := semver.NewVersion(b)
	if err != nil {
		return false, err
	}

	return sa.LessThan(sb), nil
}

// indent appends the indent string to the right of input string.
// Typically used for fixing yaml configs.
func indent(input string, indent string) string {
	return strings.ReplaceAll(input, "\n", "\n"+indent)
}
