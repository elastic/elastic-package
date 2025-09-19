// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/serverless"
	"github.com/elastic/elastic-package/internal/wait"
)

const (
	paramServerlessProjectID   = "serverless_project_id"
	paramServerlessProjectType = "serverless_project_type"
	ParamServerlessFleetURL    = "serverless_fleet_url"

	ParamServerlessLocalStackVersion = "serverless_local_stack_version"

	configRegion          = "stack.serverless.region"
	configProjectType     = "stack.serverless.type"
	configElasticCloudURL = "stack.elastic_cloud.host"

	defaultRegion      = "aws-us-east-1"
	defaultProjectType = "observability"

	defaultRetriesDefaultFleetServerPeriod  = 2 * time.Second
	defaultRetriesDefaultFleetServerTimeout = 10 * time.Second
)

var allowedProjectTypes = []string{
	"security",
	"observability",
}

type serverlessProvider struct {
	profile *profile.Profile
	client  *serverless.Client

	elasticsearchClient *elasticsearch.Client
	kibanaClient        *kibana.Client

	retriesDefaultFleetServerTimeout time.Duration
	retriesDefaultFleetServerPeriod  time.Duration
}

type projectSettings struct {
	Name   string
	Region string
	Type   string

	StackVersion    string
	LogstashEnabled bool
	SelfMonitor     bool
}

func (sp *serverlessProvider) createProject(ctx context.Context, settings projectSettings, options Options, conf Config) (Config, error) {
	project, err := sp.client.CreateProject(ctx, settings.Name, settings.Region, settings.Type)
	if err != nil {
		return Config{}, fmt.Errorf("failed to create %s project %s in %s: %w", settings.Type, settings.Name, settings.Region, err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute*30)
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

	// add stack version set in command line
	config.Parameters[ParamServerlessLocalStackVersion] = options.StackVersion

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

	found, err := wait.UntilTrue(ctx, func(ctx context.Context) (bool, error) {
		config.Parameters[ParamServerlessFleetURL], err = project.DefaultFleetServerURL(ctx, sp.kibanaClient)
		if errors.Is(err, kibana.ErrFleetServerNotFound) {
			logger.Debug("Fleet Server URL not found yet, retrying...")
			return false, nil
		}
		if err != nil {
			return false, err
		}
		logger.Debug("Fleet Server found")
		return true, nil
	}, sp.retriesDefaultFleetServerPeriod, sp.retriesDefaultFleetServerTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("error while waiting for Fleet Server URL: %w", err)
	}
	if !found {
		return Config{}, fmt.Errorf("not found Fleet Server URL after %s", sp.retriesDefaultFleetServerTimeout)
	}

	project.Endpoints.Fleet = config.Parameters[ParamServerlessFleetURL]

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

	if settings.LogstashEnabled {
		err = addLogstashFleetOutput(ctx, sp.kibanaClient)
		if err != nil {
			return Config{}, err
		}
	}

	return config, nil
}

func (sp *serverlessProvider) deleteProject(ctx context.Context, project *serverless.Project, options Options) error {
	return sp.client.DeleteProject(ctx, project)
}

func (sp *serverlessProvider) currentProjectWithClientsAndFleetEndpoint(ctx context.Context, config Config) (*serverless.Project, error) {
	project, err := sp.currentProject(ctx, config)
	if err != nil {
		return nil, err
	}

	err = sp.createClients(project)
	if err != nil {
		return nil, err
	}

	fleetURL, found := config.Parameters[ParamServerlessFleetURL]
	if !found {
		fleetURL, err = project.DefaultFleetServerURL(ctx, sp.kibanaClient)
		if err != nil {
			return nil, fmt.Errorf("failed to get fleet URL: %w", err)
		}
	}
	project.Endpoints.Fleet = fleetURL

	return project, nil
}

func (sp *serverlessProvider) currentProject(ctx context.Context, config Config) (*serverless.Project, error) {
	projectID, found := config.Parameters[paramServerlessProjectID]
	if !found {
		return nil, serverless.ErrProjectNotExist
	}

	projectType, found := config.Parameters[paramServerlessProjectType]
	if !found {
		return nil, serverless.ErrProjectNotExist
	}

	project, err := sp.client.GetProject(ctx, projectType, projectID)
	if errors.Is(err, serverless.ErrProjectNotExist) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("couldn't check project health: %w", err)
	}

	project.Credentials.Username = config.ElasticsearchUsername
	project.Credentials.Password = config.ElasticsearchPassword

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
		Name:            createProjectName(options),
		Type:            options.Profile.Config(configProjectType, defaultProjectType),
		Region:          options.Profile.Config(configRegion, defaultRegion),
		StackVersion:    options.StackVersion,
		LogstashEnabled: options.Profile.Config(configLogstashEnabled, "false") == "true",
		SelfMonitor:     options.Profile.Config(configSelfMonitorEnabled, "false") == "true",
	}

	return s, nil
}

func createProjectName(options Options) string {
	return fmt.Sprintf("elastic-package-test-%s", options.Profile.ProfileName)
}

func newServerlessProvider(profile *profile.Profile) (*serverlessProvider, error) {
	host := profile.Config(configElasticCloudURL, "")
	options := []serverless.ClientOption{}
	if host != "" {
		options = append(options, serverless.WithAddress(host))
	}
	client, err := serverless.NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("can't create serverless provider: %w", err)
	}

	return &serverlessProvider{
		profile:                          profile,
		client:                           client,
		elasticsearchClient:              nil,
		kibanaClient:                     nil,
		retriesDefaultFleetServerTimeout: defaultRetriesDefaultFleetServerTimeout,
		retriesDefaultFleetServerPeriod:  defaultRetriesDefaultFleetServerPeriod,
	}, nil
}

func (sp *serverlessProvider) BootUp(ctx context.Context, options Options) error {
	logger.Warn("Elastic Serverless provider is in technical preview")

	config, err := LoadConfig(sp.profile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	settings, err := getProjectSettings(options)
	if err != nil {
		return err
	}

	if !slices.Contains(allowedProjectTypes, settings.Type) {
		return fmt.Errorf("serverless project type not supported: %s", settings.Type)
	}

	var project *serverless.Project

	isNewProject := false
	project, err = sp.currentProject(ctx, config)
	switch err {
	default:
		return err
	case serverless.ErrProjectNotExist:
		logger.Infof("Creating %s project: %q", settings.Type, settings.Name)
		config, err = sp.createProject(ctx, settings, options, config)
		if err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}

		outputID := ""
		if settings.LogstashEnabled {
			outputID = serverless.FleetLogstashOutput
		}

		logger.Infof("Creating agent policy")
		_, err = createAgentPolicy(ctx, sp.kibanaClient, options.StackVersion, outputID, settings.SelfMonitor)
		if err != nil {
			return fmt.Errorf("failed to create agent policy: %w", err)
		}
		isNewProject = true

		// TODO: Ensuring a specific GeoIP database would make tests reproducible
		// Currently geo ip files would be ignored when running pipeline tests
	case nil:
		logger.Debugf("%s project existed: %s", project.Type, project.Name)
		printUserConfig(options.Printer, config)
	}

	logger.Infof("Starting local services")
	err = sp.startLocalServices(ctx, options, config)
	if err != nil {
		return fmt.Errorf("failed to start local services: %w", err)
	}

	// Updating the output with ssl certificates created in startLocalServices
	// The certificates are updated only when a new project is created and logstash is enabled
	if isNewProject && settings.LogstashEnabled {
		err = updateLogstashFleetOutput(ctx, sp.profile, sp.kibanaClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sp *serverlessProvider) composeProjectName() string {
	return DockerComposeProjectName(sp.profile)
}

func (sp *serverlessProvider) localServicesComposeProject() (*compose.Project, error) {
	composeFile := sp.profile.Path(ProfileStackPath, ComposeFile)
	return compose.NewProject(sp.composeProjectName(), composeFile)
}

func (sp *serverlessProvider) startLocalServices(ctx context.Context, options Options, config Config) error {
	err := applyLocalResources(sp.profile, options.StackVersion, config)
	if err != nil {
		return fmt.Errorf("could not initialize compose files for local services: %w", err)
	}

	project, err := sp.localServicesComposeProject()
	if err != nil {
		return fmt.Errorf("could not initialize local services compose project")
	}

	opts := compose.CommandOptions{
		ExtraArgs: []string{},
	}
	err = project.Build(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to build images for local services: %w", err)
	}

	if options.DaemonMode {
		opts.ExtraArgs = append(opts.ExtraArgs, "-d")
	}
	if err := project.Up(ctx, opts); err != nil {
		// At least starting on 8.6.0, fleet-server may be reconfigured or
		// restarted after being healthy. If elastic-agent tries to enroll at
		// this moment, it fails inmediately, stopping and making `docker-compose up`
		// to fail too.
		// As a workaround, try to give another chance to docker-compose if only
		// elastic-agent failed.
		if onlyElasticAgentFailed(ctx, options) && !errors.Is(err, context.Canceled) {
			fmt.Println("Elastic Agent failed to start, trying again.")
			if err := project.Up(ctx, opts); err != nil {
				return fmt.Errorf("failed to start local services: %w", err)
			}
		}
	}

	return nil
}

func (sp *serverlessProvider) TearDown(ctx context.Context, options Options) error {
	config, err := LoadConfig(sp.profile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	var errs error

	err = sp.destroyLocalServices(ctx)
	if err != nil {
		logger.Errorf("failed to destroy local services: %v", err)
		errs = fmt.Errorf("failed to destroy local services: %w", err)
	}

	project, err := sp.currentProject(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to find current project: %w", err)
	}

	logger.Debugf("Deleting project %q (%s)", project.Name, project.ID)

	err = sp.deleteProject(ctx, project, options)
	if err != nil {
		logger.Errorf("failed to delete project: %v", err)
		errs = errors.Join(errs, fmt.Errorf("failed to delete project: %w", err))
	}
	logger.Infof("Project %s (%s) deleted", project.Name, project.ID)

	// TODO: if GeoIP database is specified, remove the geoip Bundle (if needed)
	return errs
}

func (sp *serverlessProvider) destroyLocalServices(ctx context.Context) error {
	project, err := sp.localServicesComposeProject()
	if err != nil {
		return fmt.Errorf("could not initialize local services compose project")
	}

	opts := compose.CommandOptions{
		// Remove associated volumes.
		ExtraArgs: []string{"--volumes", "--remove-orphans"},
	}
	err = project.Down(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to destroy local services: %w", err)
	}

	return nil
}

func (sp *serverlessProvider) Update(ctx context.Context, options Options) error {
	return fmt.Errorf("not implemented")
}

func (sp *serverlessProvider) Dump(ctx context.Context, options DumpOptions) ([]DumpResult, error) {
	for _, service := range options.Services {
		if service != "elastic-agent" {
			return nil, &ErrNotImplemented{
				Operation: fmt.Sprintf("logs dump for service %s", service),
				Provider:  ProviderServerless,
			}
		}
	}
	return Dump(ctx, options)
}

func (sp *serverlessProvider) Status(ctx context.Context, options Options) ([]ServiceStatus, error) {
	logger.Warn("Elastic Serverless provider is in technical preview")
	config, err := LoadConfig(sp.profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	project, err := sp.currentProjectWithClientsAndFleetEndpoint(ctx, config)
	if errors.Is(err, serverless.ErrProjectNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	projectServiceStatus, err := project.Status(ctx, sp.elasticsearchClient, sp.kibanaClient)
	if err != nil {
		return nil, err
	}

	serverlessVersion := fmt.Sprintf("serverless (%s)", project.Type)
	var serviceStatus []ServiceStatus
	for service, status := range projectServiceStatus {
		serviceStatus = append(serviceStatus, ServiceStatus{
			Name:    service,
			Version: serverlessVersion,
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
		serviceName := containerDescription.Config.Labels.ComposeService
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
