// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack/serverless"
)

const (
	paramServerlessProjectID   = "serverless_project_id"
	paramServerlessProjectType = "serverless_project_type"

	configRegion      = "stack.serverless.region"
	configProjectType = "stack.serverless.type"

	defaultRegion      = "aws-eu-west-1"
	defaultProjectType = "observability"
)

var (
	errProjectNotExist = errors.New("project does not exist")
)

type serverlessProvider struct {
	profile *profile.Profile
	client  *serverless.Client
}

type projectSettings struct {
	Name   string
	Region string
	Type   string

	StackVersion string
}

func (sp *serverlessProvider) createProject(settings projectSettings, options Options) (Config, error) {
	project, err := sp.client.CreateProject(settings.Name, settings.Region, settings.Type)
	if err != nil {
		return Config{}, fmt.Errorf("failed to create %s project %s in %s: %w", settings.Type, settings.Name, settings.Region, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*30)
	defer cancel()
	if err := sp.client.EnsureEndpoints(ctx, project); err != nil {
		return Config{}, fmt.Errorf("failed to ensure endpoints have been provisioned properly: %w", err)
	}

	var config Config
	config.Provider = ProviderServerless
	config.Parameters = map[string]string{
		paramServerlessProjectID:   project.ID,
		paramServerlessProjectType: project.Type,
	}

	config.ElasticsearchHost = project.Endpoints.Elasticsearch
	config.KibanaHost = project.Endpoints.Kibana

	config.ElasticsearchUsername = project.Credentials.Username
	config.ElasticsearchPassword = project.Credentials.Password

	printUserConfig(options.Printer, config)

	err = storeConfig(sp.profile, config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to store config: %w", err)
	}

	logger.Debug("Waiting for creation plan to be completed")
	err = project.EnsureHealthy(ctx)
	if err != nil {
		return Config{}, fmt.Errorf("not all services are healthy: %w", err)
	}

	return config, nil
}

func (sp *serverlessProvider) deleteProject(project *serverless.Project, options Options) error {
	return sp.client.DeleteProject(project)
}

func (sp *serverlessProvider) currentProject(config Config) (*serverless.Project, error) {
	projectID, found := config.Parameters[paramServerlessProjectID]
	if !found {
		return nil, errProjectNotExist
	}

	projectType, found := config.Parameters[paramServerlessProjectType]
	if !found {
		return nil, errProjectNotExist
	}

	project, err := sp.client.GetProject(projectType, projectID)
	if err == serverless.ErrProjectNotExist {
		return nil, errProjectNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("couldn't check project health: %w", err)
	}

	return project, nil
}

func getProjectSettings(options Options) (projectSettings, error) {
	s := projectSettings{
		Name:         createProjectName(options),
		Type:         options.Profile.Config(configProjectType, defaultProjectType),
		Region:       options.Profile.Config(configRegion, defaultRegion),
		StackVersion: options.StackVersion,
	}

	return s, nil
}

func createProjectName(options Options) string {
	return fmt.Sprintf("elastic-package-test-%s", options.Profile.ProfileName)
}

func newServerlessProvider(profile *profile.Profile) (*serverlessProvider, error) {
	client, err := serverless.NewClient()
	if err != nil {
		return nil, fmt.Errorf("can't create serverless provider: %w", err)
	}

	return &serverlessProvider{profile, client}, nil
}

func (sp *serverlessProvider) BootUp(options Options) error {
	logger.Warn("Elastic Serverless provider is in technical preview")

	config, err := LoadConfig(sp.profile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	settings, err := getProjectSettings(options)
	if err != nil {
		return err
	}

	var project *serverless.Project

	project, err = sp.currentProject(config)
	switch err {
	default:
		return err
	case errProjectNotExist:
		logger.Infof("Creating project %q", settings.Name)
		config, err = sp.createProject(settings, options)
		if err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}

		// logger.Infof("Creating agent policy")
		// err = sp.createAgentPolicy(config, options.StackVersion)
		// if err != nil {
		// 	return fmt.Errorf("failed to create agent policy: %w", err)
		// }

		// logger.Infof("Replacing GeoIP databases")
		// err = cp.replaceGeoIPDatabases(config, options, settings.TemplateID, settings.Region, payload.Resources.Elasticsearch[0].Plan.ClusterTopology)
		// if err != nil {
		// 	return fmt.Errorf("failed to replace GeoIP databases: %w", err)
		// }
		logger.Debugf("Project created: %s", project.Name)
		printUserConfig(options.Printer, config)
	case nil:
		logger.Debugf("Project existed: %s", project.Name)
		printUserConfig(options.Printer, config)
		logger.Infof("Updating project %s", project.Name)
		// err = sp.updateDeployment(project, settings)
		// if err != nil {
		// 	return fmt.Errorf("failed to update deployment: %w", err)
		// }
	}

	// logger.Infof("Starting local agent")
	// err = sp.startLocalAgent(options, config)
	// if err != nil {
	// 	return fmt.Errorf("failed to start local agent: %w", err)
	// }

	return nil
}

// func (sp *serverlessProvider) startLocalAgent(options Options, config Config) error {
// 	err := applyCloudResources(sp.profile, options.StackVersion, config)
// 	if err != nil {
// 		return fmt.Errorf("could not initialize compose files for local agent: %w", err)
// 	}
//
// 	project, err := sp.localAgentComposeProject()
// 	if err != nil {
// 		return fmt.Errorf("could not initialize local agent compose project")
// 	}
//
// 	err = project.Build(compose.CommandOptions{})
// 	if err != nil {
// 		return fmt.Errorf("failed to build images for local agent: %w", err)
// 	}
//
// 	err = project.Up(compose.CommandOptions{ExtraArgs: []string{"-d"}})
// 	if err != nil {
// 		return fmt.Errorf("failed to start local agent: %w", err)
// 	}
//
// 	return nil
// }

func (sp *serverlessProvider) TearDown(options Options) error {
	config, err := LoadConfig(sp.profile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// err = cp.destroyLocalAgent()
	// if err != nil {
	// 	return fmt.Errorf("failed to destroy local agent: %w", err)
	// }

	project, err := sp.currentProject(config)
	if err != nil {
		return fmt.Errorf("failed to find current project: %w", err)
	}

	logger.Debugf("Deleting project %q (%s)", project.Name, project.ID)

	err = sp.deleteProject(project, options)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	// logger.Debugf("Deleting GeoIP bundle.")
	// err = cp.deleteGeoIPExtension()
	// if err != nil {
	// 	return fmt.Errorf("failed to delete GeoIP extension: %w", err)
	// }

	// err = storeConfig(sp.profile, Config{})
	// if err != nil {
	// 	return fmt.Errorf("failed to store config: %w", err)
	// }

	return nil
}

func (sp *serverlessProvider) Update(options Options) error {
	return fmt.Errorf("not implemented")
}

func (sp *serverlessProvider) Dump(options DumpOptions) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (sp *serverlessProvider) Status(options Options) ([]ServiceStatus, error) {
	logger.Warn("Elastic Serverless provider is in technical preview")
	return Status(options)
}
