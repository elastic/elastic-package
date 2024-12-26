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
	localStackResources = []resource.Resource{
		&resource.File{
			Path:    ComposeFile,
			Content: staticSource.Template("_static/local-services-docker-compose.yml.tmpl"),
		},
		&resource.File{
			Path:    ElasticAgentEnvFile,
			Content: staticSource.Template("_static/elastic-agent.env.tmpl"),
		},
	}
)

// applyLocalResources creates the local resources needed to run system tests when the stack
// is not local.
func applyLocalResources(profile *profile.Profile, stackVersion string, config Config) error {
	appConfig, err := install.Configuration(install.OptionWithStackVersion(stackVersion))
	if err != nil {
		return fmt.Errorf("can't read application configuration: %w", err)
	}
	imageRefs := appConfig.StackImageRefs()

	stackDir := filepath.Join(profile.ProfilePath, ProfileStackPath)

	resourceManager := resource.NewManager()
	resourceManager.AddFacter(resource.StaticFacter{
		"agent_version":      stackVersion,
		"agent_image":        imageRefs.ElasticAgent,
		"logstash_image":     imageRefs.Logstash,
		"elasticsearch_host": esHostWithPort(config.ElasticsearchHost),
		"api_key":            config.ElasticsearchAPIKey, // TODO: !! We will need to enroll with an enrollment token?
		"username":           config.ElasticsearchUsername,
		"password":           config.ElasticsearchPassword,
		"kibana_host":        config.KibanaHost,
		"fleet_url":          config.Parameters[ParamServerlessFleetURL],
		"logstash_enabled":   profile.Config("stack.logstash_enabled", "false"),
	})

	os.MkdirAll(stackDir, 0755)
	resourceManager.RegisterProvider("file", &resource.FileProvider{
		Prefix: stackDir,
	})

	resources := append([]resource.Resource{}, localStackResources...)

	// Keeping certificates in the profile directory for backwards compatibility reasons.
	resourceManager.RegisterProvider("certs", &resource.FileProvider{
		Prefix: profile.ProfilePath,
	})
	certResources, err := initTLSCertificates("certs", profile.ProfilePath, tlsLocalServices)
	if err != nil {
		return fmt.Errorf("failed to create TLS files: %w", err)
	}
	resources = append(resources, certResources...)

	// Add related resources and client certificates if logstash is enabled.
	if profile.Config("stack.logstash_enabled", "false") == "true" {
		resources = append(resources, logstashResources...)
	}

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
