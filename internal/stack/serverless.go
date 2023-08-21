// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/elasticsearch"
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

	config.Parameters[paramServerlessFleetURL], err = project.DefaultFleetServerURL(ctx)
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

	fleetURL, ok := config.Parameters[paramServerlessFleetURL]
	if !ok {
		fleetURL, err = project.DefaultFleetServerURL(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get fleet URL: %w", err)
		}
	}
	project.Endpoints.Fleet = fleetURL

	project, err = sp.createClients(project)
	if err != nil {
		return nil, fmt.Errorf("failed to create project client")
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

		logger.Infof("Creating agent policy")
		err = sp.createAgentPolicy(config, options.StackVersion)
		if err != nil {
			return fmt.Errorf("failed to create agent policy: %w", err)
		}

		// logger.Infof("Replacing GeoIP databases")
		// err = cp.replaceGeoIPDatabases(config, options, settings.TemplateID, settings.Region, payload.Resources.Elasticsearch[0].Plan.ClusterTopology)
		// if err != nil {
		// 	return fmt.Errorf("failed to replace GeoIP databases: %w", err)
		// }
	case nil:
		logger.Debugf("Project existed: %s", project.Name)
		printUserConfig(options.Printer, config)
		logger.Infof("Updating project %s", project.Name)
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

const serverlessKibanaAgentPolicy = `{
  "name": "Elastic-Agent (elastic-package)",
  "id": "elastic-agent-managed-ep",
  "description": "Policy created by elastic-package",
  "namespace": "default",
  "monitoring_enabled": [
    "logs",
    "metrics"
  ]
}`

const serverlessKibanaPackagePolicy = `{
  "name": "system-1",
  "policy_id": "elastic-agent-managed-ep",
  "package": {
    "name": "system",
    "version": "%s"
  }
}`

func doKibanaRequest(config Config, req *http.Request) error {
	req.SetBasicAuth(config.ElasticsearchUsername, config.ElasticsearchPassword)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("kbn-xsrf", "elastic-package")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		// Already created, go on.
		// TODO: We could try to update the policy.
		return nil
	}
	if resp.StatusCode >= 300 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("request failed with status %v and could not read body: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("request failed with status %v and response %v", resp.StatusCode, string(body))
	}
	return nil
}

func (sp *serverlessProvider) createAgentPolicy(config Config, stackVersion string) error {
	agentPoliciesURL, err := url.JoinPath(config.KibanaHost, "/api/fleet/agent_policies")
	if err != nil {
		return fmt.Errorf("failed to build url for agent policies: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, agentPoliciesURL, strings.NewReader(serverlessKibanaAgentPolicy))
	if err != nil {
		return fmt.Errorf("failed to initialize request to create agent policy: %w", err)
	}
	err = doKibanaRequest(config, req)
	if err != nil {
		return fmt.Errorf("error while creating agent policy: %w", err)
	}

	systemVersion, err := getPackageVersion("https://epr.elastic.co", "system", stackVersion)
	if err != nil {
		return fmt.Errorf("could not get the system package version for kibana %v: %w", stackVersion, err)
	}

	packagePoliciesURL, err := url.JoinPath(config.KibanaHost, "/api/fleet/package_policies")
	if err != nil {
		return fmt.Errorf("failed to build url for package policies: %w", err)
	}
	packagePolicy := fmt.Sprintf(serverlessKibanaPackagePolicy, systemVersion)
	req, err = http.NewRequest(http.MethodPost, packagePoliciesURL, strings.NewReader(packagePolicy))
	if err != nil {
		return fmt.Errorf("failed to initialize request to create package policy: %w", err)
	}
	err = doKibanaRequest(config, req)
	if err != nil {
		return fmt.Errorf("error while creating package policy: %w", err)
	}

	return nil
}

func getPackageVersion(registryURL, packageName, stackVersion string) (string, error) {
	searchURL, err := url.JoinPath(registryURL, "search")
	if err != nil {
		return "", fmt.Errorf("could not build URL: %w", err)
	}
	searchURL = fmt.Sprintf("%s?package=%s&kibana.version=%s", searchURL, packageName, stackVersion)
	resp, err := http.Get(searchURL)
	if err != nil {
		return "", fmt.Errorf("request failed (url: %s): %w", searchURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status code %v", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	var packages []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	err = json.Unmarshal(body, &packages)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}
	if len(packages) != 1 {
		return "", fmt.Errorf("expected 1 package, obtained %v", len(packages))
	}
	if found := packages[0].Name; found != packageName {
		return "", fmt.Errorf("expected package %s, found %s", packageName, found)
	}

	return packages[0].Version, nil
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
	return "", fmt.Errorf("not implemented")
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
	// query directly to docker to avoid load environment variables (e.g. STACK_VERSION_VARIANT) and profiles
	containerIDs, err := docker.ContainerIDsWithLabel(projectLabelDockerCompose, sp.composeProjectName())
	if err != nil {
		return nil, err
	}

	if len(containerIDs) == 0 {
		return services, nil
	}

	containerDescriptions, err := docker.InspectContainers(containerIDs...)
	if err != nil {
		return nil, err
	}

	for _, containerDescription := range containerDescriptions {
		service, err := newServiceStatus(&containerDescription)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(service.Name, readyServicesSuffix) {
			continue
		}
		logger.Debugf("Adding Service: \"%v\"", service.Name)
		services = append(services, *service)
	}

	return services, nil
}
