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

	defaultRegion      = "aws-us-east-1"
	defaultProjectType = "observability"
)

var (
	allowedProjectTypes = map[string]struct{}{
		"security":      {},
		"observability": {},
	}
)

type serverlessProvider struct {
	profile *profile.Profile
	client  *serverless.Client

	elasticsearchClient *elasticsearch.Client
	kibanaClient        *kibana.Client
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

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*30)
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

	err = sp.createClients(project)
	if err != nil {
		return Config{}, err
	}

	config.Parameters[paramServerlessFleetURL], err = project.DefaultFleetServerURL(sp.kibanaClient)
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

	err = project.EnsureHealthy(ctx, sp.elasticsearchClient, sp.kibanaClient)
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
		return nil, fmt.Errorf("mssing serverless project id")
	}

	projectType, found := config.Parameters[paramServerlessProjectType]
	if !found {
		return nil, fmt.Errorf("missing serverless project type")
	}

	project, err := sp.client.GetProject(projectType, projectID)
	if errors.Is(serverless.ErrProjectNotExist, err) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("couldn't check project health: %w", err)
	}

	project.Credentials.Username = config.ElasticsearchUsername
	project.Credentials.Password = config.ElasticsearchPassword

	err = sp.createClients(project)
	if err != nil {
		return nil, err
	}

	fleetURL := config.Parameters[paramServerlessFleetURL]
	if true {
		fleetURL, err = project.DefaultFleetServerURL(sp.kibanaClient)
		if err != nil {
			return nil, fmt.Errorf("failed to get fleet URL: %w", err)
		}
	}
	project.Endpoints.Fleet = fleetURL

	return project, nil
}

func (sp *serverlessProvider) createClients(project *serverless.Project) error {
	var err error
	sp.elasticsearchClient, err = NewElasticsearchClient(
		elasticsearch.OptionWithAddress(project.Endpoints.Elasticsearch),
		elasticsearch.OptionWithUsername(project.Credentials.Username),
		elasticsearch.OptionWithPassword(project.Credentials.Password),
	)
	if err != nil {
		return fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	sp.kibanaClient, err = NewKibanaClient(
		kibana.Address(project.Endpoints.Kibana),
		kibana.Username(project.Credentials.Username),
		kibana.Password(project.Credentials.Password),
	)
	if err != nil {
		return fmt.Errorf("failed to create kibana client: %w", err)
	}

	return nil
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

	return &serverlessProvider{profile, client, nil, nil}, nil
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

	_, ok := allowedProjectTypes[settings.Type]
	if !ok {
		return fmt.Errorf("serverless project type not supported: %s", settings.Type)
	}

	var project *serverless.Project

	project, err = sp.currentProject(config)
	switch err {
	default:
		return err
	case serverless.ErrProjectNotExist:
		logger.Infof("Creating %s project: %q", settings.Type, settings.Name)
		config, err = sp.createProject(settings, options, config)
		if err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}

		project, err = sp.currentProject(config)
		if err != nil {
			return fmt.Errorf("failed to retrieve latest project created: %w", err)
		}

		err = sp.createClients(project)
		if err != nil {
			return err
		}

		logger.Infof("Creating agent policy")
		err = project.CreateAgentPolicy(options.StackVersion, sp.kibanaClient)
		if err != nil {
			return fmt.Errorf("failed to create agent policy: %w", err)
		}

		// TODO: Ensuring a specific GeoIP database would make tests reproducible
		// Currently geo ip files would be ignored when running pipeline tests
	case nil:
		logger.Debugf("%s project existed: %s", project.Type, project.Name)
		printUserConfig(options.Printer, config)
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

	// TODO: if GeoIP database is specified, remove the geoip Bundle (if needed)
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
	if errors.Is(serverless.ErrProjectNotExist, err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	ctx := context.TODO()
	projectServiceStatus, err := project.Status(ctx, sp.elasticsearchClient, sp.kibanaClient)
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
