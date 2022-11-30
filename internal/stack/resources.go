// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/go-resource"
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

	// KibanaHealthcheckFile is the kibana healthcheck.
	KibanaHealthcheckFile = "kibana_healthcheck.sh"

	// PackageRegistryConfigFile is the config file for the Elastic Package registry
	PackageRegistryConfigFile = "package-registry.yml"

	// PackageRegistryBaseImage is the base Docker image of the Elastic Package Registry.
	PackageRegistryBaseImage = "docker.elastic.co/package-registry/package-registry:v1.15.0"

	// ElasticAgentEnvFile is the elastic agent environment variables file.
	ElasticAgentEnvFile = "elastic-agent.env"

	profileStackPath = "stack"
)

var (
	templateFuncs = template.FuncMap{
		"semverLessThan": semverLessThan,
	}
	staticSource   = resource.NewSourceFS(static).WithTemplateFuncs(templateFuncs)
	stackResources = []resource.Resource{
		&resource.File{
			Path:    "Dockerfile.package-registry",
			Content: staticSource.Template("_static/Dockerfile.package-registry.tmpl"),
		},
		&resource.File{
			Path:    SnapshotFile,
			Content: staticSource.File("_static/docker-compose-stack.yml"),
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
			Path:    KibanaHealthcheckFile,
			Content: staticSource.File("_static/kibana_healthcheck.sh"),
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
	})

	os.MkdirAll(stackDir, 0755)
	resourceManager.RegisterProvider("file", &resource.FileProvider{
		Prefix: stackDir,
	})

	resources := append([]resource.Resource{}, stackResources...)

	certResources, err := initTLSCertificates("file", profile.ProfilePath)
	if err != nil {
		return fmt.Errorf("failed to create TLS files: %w", err)
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
