// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/go-resource"
)

const (
	// ServerlessElasticAgentEnvFile is the elastic agent environment variables file for the
	// serverless provider.
	ServerlessElasticAgentEnvFile = "serverless-elastic-agent.env"

	// ServerlessComposeFile is the docker-compose snapshot.yml file name.
	ServerlessComposeFile = "serverless-elastic-agent.yml"
)

var (
	serverlessStackResources = []resource.Resource{
		&resource.File{
			Path:    ServerlessComposeFile,
			Content: staticSource.Template("_static/serverless-elastic-agent.yml.tmpl"),
		},
		&resource.File{
			Path:    ServerlessElasticAgentEnvFile,
			Content: staticSource.Template("_static/serverless-elastic-agent.env.tmpl"),
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
		"agent_version": stackVersion,
		"agent_image":   appConfig.StackImageRefs(stackVersion).ElasticAgent,
		"username":      config.ElasticsearchUsername,
		"password":      config.ElasticsearchPassword,
		"kibana_host":   config.KibanaHost,
		"fleet_url":     config.Parameters[paramServerlessFleetURL],
	})

	os.MkdirAll(stackDir, 0755)
	resourceManager.RegisterProvider("file", &resource.FileProvider{
		Prefix: stackDir,
	})

	results, err := resourceManager.Apply(serverlessStackResources)
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
