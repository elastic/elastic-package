// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/serverless"
)

const (
	paramServerlessProjectID   = "serverless_project_id"
	paramServerlessProjectType = "serverless_project_type"
	paramServerlessFleetURL    = "serverless_fleet_url"

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

func (sp *serverlessProvider) createProject(settings projectSettings, options Options, conf Config) (Config, error) {
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

	// Store config now in case fails initialization or other requests,
	// so it can be destroyed later
	err = storeConfig(sp.profile, config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to store config: %w", err)
	}

	logger.Debug("Waiting for creation plan to be completed")
	err = sp.client.EnsureProjectInitialized(ctx, project)
	if err != nil {
		return Config{}, fmt.Errorf("project not initialized: %w", err)
	}

	project, err = sp.createClients(project)
	if err != nil {
		return Config{}, fmt.Errorf("failed to create project client")
	}

	config.Parameters[paramServerlessFleetURL], err = project.DefaultFleetServerURL()
	if err != nil {
		return Config{}, fmt.Errorf("failed to get fleet URL: %w", err)
	}
	project.Endpoints.Fleet = config.Parameters[paramServerlessFleetURL]

	printUserConfig(options.Printer, config)

	// update config with latest updates (e.g. fleet server url)
	err = storeConfig(sp.profile, config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to store config: %w", err)
	}

	err = project.EnsureHealthy(ctx)
	if err != nil {
		return Config{}, fmt.Errorf("not all services are healthy: %w", err)
	}

	return config, nil
}

func (sp *serverlessProvider) createClients(project *serverless.Project) (*serverless.Project, error) {
	var err error
	project.ElasticsearchClient, err = NewElasticsearchClient(
		elasticsearch.OptionWithAddress(project.Endpoints.Elasticsearch),
		elasticsearch.OptionWithUsername(project.Credentials.Username),
		elasticsearch.OptionWithPassword(project.Credentials.Password),
	)
	if err != nil {
		return project, fmt.Errorf("failed to create elasticsearch client")
	}

	project.KibanaClient, err = NewKibanaClient(
		kibana.Address(project.Endpoints.Kibana),
		kibana.Username(project.Credentials.Username),
		kibana.Password(project.Credentials.Password),
	)
	if err != nil {
		return project, fmt.Errorf("failed to create kibana client")
	}

	return project, nil
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

	project.Credentials.Username = config.ElasticsearchUsername
	project.Credentials.Password = config.ElasticsearchPassword

	project, err = sp.createClients(project)
	if err != nil {
		return nil, fmt.Errorf("failed to create project client")
	}

	fleetURL := config.Parameters[paramServerlessFleetURL]
	if true {
		fleetURL, err = project.DefaultFleetServerURL()
		if err != nil {
			return nil, fmt.Errorf("failed to get fleet URL: %w", err)
		}
	}
	project.Endpoints.Fleet = fleetURL

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

	if settings.Type == "elasticsearch" {
		return fmt.Errorf("serverless project type not supported: %s", settings.Type)
	}

	var project *serverless.Project

	project, err = sp.currentProject(config)
	switch err {
	default:
		return err
	case errProjectNotExist:
		logger.Infof("Creating %s project: %q", settings.Type, settings.Name)
		config, err = sp.createProject(settings, options, config)
		if err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}

		project, err = sp.currentProject(config)
		if err != nil {
			return fmt.Errorf("failed to retrieve latest project created: %w", err)
		}

		logger.Infof("Creating agent policy")
		err = project.CreateAgentPolicy(options.StackVersion)
		if err != nil {
			return fmt.Errorf("failed to create agent policy: %w", err)
		}

		// logger.Infof("Replacing GeoIP databases")
		// err = cp.replaceGeoIPDatabases(config, options, settings.TemplateID, settings.Region, payload.Resources.Elasticsearch[0].Plan.ClusterTopology)
		// if err != nil {
		// 	return fmt.Errorf("failed to replace GeoIP databases: %w", err)
		// }
	case nil:
		logger.Debugf("%s project existed: %s", project.Type, project.Name)
		printUserConfig(options.Printer, config)
		// logger.Infof("Updating project %s", project.Name)
		// err = sp.updateDeployment(project, settings)
		// if err != nil {
		// 	return fmt.Errorf("failed to update deployment: %w", err)
		// }
	}

	logger.Infof("Starting local agent")
	err = sp.startLocalAgent(options, config)
	if err != nil {
		return fmt.Errorf("failed to start local agent: %w", err)
	}

	return nil
}

func (sp *serverlessProvider) composeProjectName() string {
	return DockerComposeProjectName(sp.profile)
}

func (sp *serverlessProvider) localAgentComposeProject() (*compose.Project, error) {
	composeFile := sp.profile.Path(profileStackPath, ServerlessComposeFile)
	return compose.NewProject(sp.composeProjectName(), composeFile)
}

func (sp *serverlessProvider) startLocalAgent(options Options, config Config) error {
	err := applyServerlessResources(sp.profile, options.StackVersion, config)
	if err != nil {
		return fmt.Errorf("could not initialize compose files for local agent: %w", err)
	}

	project, err := sp.localAgentComposeProject()
	if err != nil {
		return fmt.Errorf("could not initialize local agent compose project")
	}

	err = project.Build(compose.CommandOptions{})
	if err != nil {
		return fmt.Errorf("failed to build images for local agent: %w", err)
	}

	err = project.Up(compose.CommandOptions{ExtraArgs: []string{"-d"}})
	if err != nil {
		return fmt.Errorf("failed to start local agent: %w", err)
	}

	return nil
}

func (sp *serverlessProvider) TearDown(options Options) error {
	config, err := LoadConfig(sp.profile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	err = sp.destroyLocalAgent()
	if err != nil {
		return fmt.Errorf("failed to destroy local agent: %w", err)
	}

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

func (sp *serverlessProvider) destroyLocalAgent() error {
	project, err := sp.localAgentComposeProject()
	if err != nil {
		return fmt.Errorf("could not initialize local agent compose project")
	}

	err = project.Down(compose.CommandOptions{})
	if err != nil {
		return fmt.Errorf("failed to destroy local agent: %w", err)
	}

	return nil
}

func (sp *serverlessProvider) Update(options Options) error {
	return fmt.Errorf("not implemented")
}

func (sp *serverlessProvider) Dump(options DumpOptions) (string, error) {
	return Dump(options)
}

func (sp *serverlessProvider) Status(options Options) ([]ServiceStatus, error) {
	logger.Warn("Elastic Serverless provider is in technical preview")
	config, err := LoadConfig(sp.profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	project, err := sp.currentProject(config)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	projectServiceStatus, err := project.Status(ctx)
	if err != nil {
		return nil, err
	}

	var serviceStatus []ServiceStatus
	for service, status := range projectServiceStatus {
		serviceStatus = append(serviceStatus, ServiceStatus{
			Name:    service,
			Version: "serverless",
			Status:  status,
		})
	}

	agentStatus, err := sp.localAgentStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get local agent status: %w", err)
	}

	serviceStatus = append(serviceStatus, agentStatus...)

	return serviceStatus, nil
}

func (sp *serverlessProvider) localAgentStatus() ([]ServiceStatus, error) {
	var services []ServiceStatus
	serviceStatusFunc := func(description docker.ContainerDescription) error {
		service, err := newServiceStatus(&description)
		if err != nil {
			return err
		}
		services = append(services, *service)
		return nil
	}

	err := runOnLocalServices(sp.composeProjectName(), serviceStatusFunc)
	if err != nil {
		return nil, err
	}

	return services, nil
}

func localServiceNames(project string) ([]string, error) {
	services := []string{}
	serviceFunc := func(description docker.ContainerDescription) error {
		services = append(services, description.Config.Labels[serviceLabelDockerCompose])
		return nil
	}

	err := runOnLocalServices(project, serviceFunc)
	if err != nil {
		return nil, err
	}

	return services, nil
}

func runOnLocalServices(project string, serviceFunc func(docker.ContainerDescription) error) error {
	// query directly to docker to avoid load environment variables (e.g. STACK_VERSION_VARIANT) and profiles
	containerIDs, err := docker.ContainerIDsWithLabel(projectLabelDockerCompose, project)
	if err != nil {
		return err
	}

	if len(containerIDs) == 0 {
		return nil
	}

	containerDescriptions, err := docker.InspectContainers(containerIDs...)
	if err != nil {
		return err
	}

	for _, containerDescription := range containerDescriptions {
		serviceName := containerDescription.Config.Labels[serviceLabelDockerCompose]
		if strings.HasSuffix(serviceName, readyServicesSuffix) {
			continue
		}
		err := serviceFunc(containerDescription)
		if err != nil {
			return err
		}
	}
	return nil
}
