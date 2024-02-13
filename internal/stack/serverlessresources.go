// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
)

var (
	serverlessStackResources = []resource.Resource{
		&resource.File{
			Path:    SnapshotFile,
			Content: staticSource.Template("_static/serverless-docker-compose.yml.tmpl"),
		},
		&resource.File{
			Path:    ElasticAgentEnvFile,
			Content: staticSource.Template("_static/elastic-agent.env.tmpl"),
		},
		&resource.File{
			Path:    LogstashConfigFile,
			Content: staticSource.Template("_static/serverless-logstash.conf.tmpl"),
		},
	}
)

func applyServerlessResources(profile *profile.Profile, stackVersion string, config Config) error {
	appConfig, err := install.Configuration()
	if err != nil {
		return fmt.Errorf("can't read application configuration: %w", err)
	}

	stackDir := filepath.Join(profile.ProfilePath, profileStackPath)

	resourceManager := resource.NewManager()
	resourceManager.AddFacter(resource.StaticFacter{
		"agent_version":      stackVersion,
		"agent_image":        appConfig.StackImageRefs(stackVersion).ElasticAgent,
		"logstash_image":     appConfig.StackImageRefs(stackVersion).Logstash,
		"elasticsearch_host": esHostWithPort(config.ElasticsearchHost),
		"username":           config.ElasticsearchUsername,
		"password":           config.ElasticsearchPassword,
		"kibana_host":        config.KibanaHost,
		"fleet_url":          config.Parameters[paramServerlessFleetURL],
		"logstash_enabled":   profile.Config("stack.logstash_enabled", "false"),
	})

	os.MkdirAll(stackDir, 0755)
	resourceManager.RegisterProvider("file", &resource.FileProvider{
		Prefix: stackDir,
	})

	resources := append([]resource.Resource{}, serverlessStackResources...)

	// Keeping certificates in the profile directory for backwards compatibility reasons.
	resourceManager.RegisterProvider("certs", &resource.FileProvider{
		Prefix: profile.ProfilePath,
	})
	certResources, err := initTLSCertificates("certs", profile.ProfilePath, tlsServicesServerless)
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

// esHostWithPort checks if the es host has a port already added in the string , else adds 443
// This is to mitigate a known issue in logstash - https://www.elastic.co/guide/en/logstash/current/plugins-outputs-elasticsearch.html#plugins-outputs-elasticsearch-serverless
func esHostWithPort(host string) string {
	url, err := url.Parse(host)
	if err != nil {
		return host
	}

	if url.Port() == "" {
		url.Host = net.JoinHostPort(url.Hostname(), "443")
		return url.String()
	}

	return host
}
