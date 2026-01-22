// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"context"
	"crypto/md5"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	dockerTestAgentServiceName   = "elastic-agent"
	dockerTestAgentDockerCompose = "docker-agent-base.yml"
	dockerTestAgentDockerfile    = "Dockerfile"
	customScriptFilename         = "script.sh"
	customEntrypointFilename     = "custom-entrypoint.sh"
	defaultAgentPolicyName       = "Elastic-Agent (elastic-package)"
)

//go:embed _static
var static embed.FS

var staticSource = resource.NewSourceFS(static)

// CustomAgentDeployer knows how to deploy a custom elastic-agent defined via
// a Docker Compose file.
type DockerComposeAgentDeployer struct {
	profile      *profile.Profile
	workDir      string
	stackVersion string

	policyName string

	agentRunID string

	packageName string
	dataStream  string

	runTearDown  bool
	runTestsOnly bool
}

type DockerComposeAgentDeployerOptions struct {
	Profile      *profile.Profile
	WorkDir      string
	StackVersion string
	PolicyName   string

	PackageName string
	DataStream  string

	RunTearDown  bool
	RunTestsOnly bool
}

var _ AgentDeployer = new(DockerComposeAgentDeployer)

type dockerComposeDeployedAgent struct {
	agentInfo AgentInfo

	ymlPaths  []string
	project   string
	env       []string
	configDir string
	workDir   string
}

var _ DeployedAgent = new(dockerComposeDeployedAgent)

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(options DockerComposeAgentDeployerOptions) (*DockerComposeAgentDeployer, error) {
	return &DockerComposeAgentDeployer{
		profile:      options.Profile,
		workDir:      options.WorkDir,
		stackVersion: options.StackVersion,
		packageName:  options.PackageName,
		dataStream:   options.DataStream,
		policyName:   options.PolicyName,
		runTearDown:  options.RunTearDown,
		runTestsOnly: options.RunTestsOnly,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *DockerComposeAgentDeployer) SetUp(ctx context.Context, agentInfo AgentInfo) (DeployedAgent, error) {
	logger.Debug("setting up agent using Docker Compose agent deployer")
	d.agentRunID = agentInfo.Test.RunID

	appConfig, err := install.Configuration(install.OptionWithStackVersion(d.stackVersion))
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	caCertPath, err := stack.FindCACertificate(d.profile)
	if err != nil {
		return nil, fmt.Errorf("can't locate CA certificate: %w", err)
	}

	env := append(
		appConfig.StackImageRefs().AsEnv(),
		fmt.Sprintf("%s=%s", serviceLogsDirEnv, agentInfo.Logs.Folder.Local),
		fmt.Sprintf("%s=%s", localCACertEnv, caCertPath),
		fmt.Sprintf("%s=%s", fleetPolicyEnv, d.policyName),
		fmt.Sprintf("%s=%s", agentHostnameEnv, d.agentHostname()),
	)

	configDir, err := d.installDockerCompose(ctx, agentInfo)
	if err != nil {
		return nil, fmt.Errorf("could not create resources for custom agent: %w", err)
	}

	composeProjectName := fmt.Sprintf("elastic-package-agent-%s-%s", d.agentName(), agentInfo.Test.RunID)

	agent := dockerComposeDeployedAgent{
		ymlPaths:  []string{filepath.Join(configDir, dockerTestAgentDockerCompose)},
		project:   composeProjectName,
		env:       env,
		configDir: configDir,
		workDir:   d.workDir,
	}

	agentInfo.NetworkName = fmt.Sprintf("%s_default", composeProjectName)

	p, err := compose.NewProject(agent.project, agent.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for agent: %w", err)
	}

	// Verify the Elastic stack network
	err = stack.EnsureStackNetworkUp(d.profile)
	if err != nil {
		return nil, fmt.Errorf("stack network is not ready: %w", err)
	}

	// Clean service logs
	if d.runTestsOnly {
		// service logs folder must no be deleted to avoid breaking log files written
		// by the service. If this is required, those files should be rotated or truncated
		// so the service can still write to them.
		logger.Debugf("Skipping removing service logs folder %s", agentInfo.Logs.Folder.Local)
	} else {
		err = files.RemoveContent(agentInfo.Logs.Folder.Local)
		if err != nil {
			return nil, fmt.Errorf("removing service logs failed: %w", err)
		}
	}

	// Service name defined in the docker-compose file
	agentInfo.Name = dockerTestAgentServiceName
	agentName := agentInfo.Name

	opts := compose.CommandOptions{
		Env:       env,
		ExtraArgs: []string{"--build", "-d"},
	}

	if d.runTestsOnly || d.runTearDown {
		logger.Debug("Skipping bringing up docker-compose project and connect container to network (non setup steps)")
	} else {
		err = p.Up(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("could not boot up agent using Docker Compose: %w", err)
		}
		// Connect service network with stack network (for the purpose of metrics collection)
		err = docker.ConnectToNetwork(p.ContainerName(agentName), stack.Network(d.profile))
		if err != nil {
			return nil, fmt.Errorf("can't attach agent container to the stack network: %w", err)
		}
	}

	// requires to be connected the service to the stack network
	err = p.WaitForHealthy(ctx, opts)
	if err != nil {
		processAgentContainerLogs(ctx, d.workDir, p, compose.CommandOptions{
			Env: opts.Env,
		}, agentName)
		return nil, fmt.Errorf("service is unhealthy: %w", err)
	}

	// Build agent container name
	// For those packages that require to do requests to agent ports in their tests (e.g. ti_anomali),
	// using the ContainerName of the agent (p.ContainerName(agentName)) as in servicedeployer does not work,
	// probably because it is in another compose project in case of ti_anomali?.
	agentInfo.Hostname = d.agentHostname()

	logger.Debugf("adding service container %s internal ports to context", p.ContainerName(agentName))
	serviceComposeConfig, err := p.Config(ctx, compose.CommandOptions{Env: env})
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for service: %w", err)
	}

	s := serviceComposeConfig.Services[agentName]
	agentInfo.Ports = make([]int, len(s.Ports))
	for idx, port := range s.Ports {
		agentInfo.Ports[idx] = port.InternalPort
	}

	// Shortcut to first port for convenience
	if len(agentInfo.Ports) > 0 {
		agentInfo.Port = agentInfo.Ports[0]
	}

	agentInfo.Agent.Host.NamePrefix = agentInfo.Name
	agent.agentInfo = agentInfo
	return &agent, nil
}

func (d *DockerComposeAgentDeployer) ProjectName(runID string) string {
	return fmt.Sprintf("elastic-package-agent-%s-%s", d.agentName(), runID)
}

func (d *DockerComposeAgentDeployer) agentHostname() string {
	return fmt.Sprintf("%s-%s", dockerTestAgentServiceName, d.agentRunID)
}

func (d *DockerComposeAgentDeployer) agentName() string {
	name := d.packageName
	if d.dataStream != "" && d.dataStream != "." {
		name = fmt.Sprintf("%s-%s", name, d.dataStream)
	}
	return name
}

// installDockerCompose creates the files needed to run the custom elastic agent and returns
// the directory with these files.
func (d *DockerComposeAgentDeployer) installDockerCompose(ctx context.Context, agentInfo AgentInfo) (string, error) {
	customAgentDir, err := CreateDeployerDir(d.profile, fmt.Sprintf("docker-agent-%s-%s", d.agentName(), d.agentRunID))
	if err != nil {
		return "", fmt.Errorf("failed to create directory for custom agent files: %w", err)
	}

	hashDockerfile := []byte{}
	if agentInfo.Agent.ProvisioningScript.Contents != "" || agentInfo.Agent.PreStartScript.Contents != "" {
		err = d.installDockerfileResources(agentInfo.Agent.AgentSettings, customAgentDir)
		if err != nil {
			return "", fmt.Errorf("failed to create dockerfile resources: %w", err)
		}
		hashDockerfile, err = hashFile(filepath.Join(customAgentDir, dockerTestAgentDockerfile))
		if err != nil {
			return "", fmt.Errorf("failed to obtain has for Elastic Agent Dockerfile: %w", err)
		}
	}
	config, err := stack.LoadConfig(d.profile)
	if err != nil {
		return "", fmt.Errorf("failed to load config from profile: %w", err)
	}
	enrollmentToken := ""
	if config.ElasticsearchAPIKey != "" {
		// TODO: Review if this is the correct place to get the enrollment token.
		kibanaClient, err := stack.NewKibanaClientFromProfile(d.profile)
		if err != nil {
			return "", fmt.Errorf("failed to create kibana client: %w", err)
		}
		enrollmentToken, err = kibanaClient.GetEnrollmentTokenForPolicyID(ctx, agentInfo.Policy.ID)
		if err != nil {
			return "", fmt.Errorf("failed to get enrollment token for policy %q: %w", agentInfo.Policy.Name, err)
		}
	}

	// TODO: Include these settings more explicitly in `config`.
	fleetURL := "https://fleet-server:8220"
	kibanaHost := "https://kibana:5601"
	stackVersion := d.stackVersion
	if config.Provider != stack.ProviderCompose {
		kibanaHost = config.KibanaHost
	}
	if url, ok := config.Parameters[stack.ParamServerlessFleetURL]; ok {
		fleetURL = url
	}
	if version, ok := config.Parameters[stack.ParamServerlessLocalStackVersion]; ok {
		stackVersion = version
	}

	agentImage, err := selectElasticAgentImage(stackVersion, agentInfo.Agent.BaseImage)
	if err != nil {
		return "", nil
	}

	resourceManager := resource.NewManager()
	resourceManager.AddFacter(resource.StaticFacter{
		"agent_image":            agentImage,
		"user":                   agentInfo.Agent.User,
		"capabilities":           strings.Join(agentInfo.Agent.LinuxCapabilities, ","),
		"runtime":                agentInfo.Agent.Runtime,
		"pid_mode":               agentInfo.Agent.PidMode,
		"ports":                  strings.Join(agentInfo.Agent.Ports, ","),
		"dockerfile_hash":        hex.EncodeToString(hashDockerfile),
		"stack_version":          stackVersion,
		"fleet_url":              fleetURL,
		"kibana_host":            stack.DockerInternalHost(kibanaHost),
		"elasticsearch_username": config.ElasticsearchUsername,
		"elasticsearch_password": config.ElasticsearchPassword,
		"enrollment_token":       enrollmentToken,
	})

	resourceManager.RegisterProvider("file", &resource.FileProvider{
		Prefix: customAgentDir,
	})

	agentResources := []resource.Resource{
		&resource.File{
			Path:    dockerTestAgentDockerCompose,
			Content: staticSource.Template("_static/docker-agent-base.yml.tmpl"),
		},
	}
	results, err := resourceManager.Apply(agentResources)
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, common.ProcessResourceApplyResults(results))
	}

	return customAgentDir, nil
}

func selectElasticAgentImage(stackVersion, agentBaseImage string) (string, error) {
	appConfig, err := install.Configuration(install.OptionWithAgentBaseImage(agentBaseImage), install.OptionWithStackVersion(stackVersion))
	if err != nil {
		return "", fmt.Errorf("can't read application configuration: %w", err)
	}

	agentImage := appConfig.StackImageRefs().ElasticAgent
	return agentImage, nil
}

func (d *DockerComposeAgentDeployer) installDockerfileResources(agentSettings AgentSettings, folder string) error {
	agentResources := []resource.Resource{
		&resource.File{
			Path:    dockerTestAgentDockerfile,
			Content: staticSource.Template("_static/dockerfile.tmpl"),
		},
	}
	if agentSettings.ProvisioningScript.Contents != "" {
		agentResources = append(agentResources, &resource.File{
			Path:    customScriptFilename,
			Mode:    resource.FileMode(0o755),
			Content: resource.FileContentLiteral(agentSettings.ProvisioningScript.Contents),
		})
	}
	if agentSettings.PreStartScript.Contents != "" {
		agentResources = append(agentResources, &resource.File{
			Path:    customEntrypointFilename,
			Mode:    resource.FileMode(0o755),
			Content: staticSource.Template("_static/custom-entrypoint.sh.tmpl"),
		})
	}
	resourceManager := resource.NewManager()
	resourceManager.AddFacter(resource.StaticFacter{
		"provisioning_script_contents": agentSettings.ProvisioningScript.Contents,
		"provisioning_script_language": agentSettings.ProvisioningScript.Language,
		"provisioning_script_filename": customScriptFilename,
		"pre_start_script_contents":    agentSettings.PreStartScript.Contents,
		"entrypoint_script_filename":   customEntrypointFilename,
		"agent_name":                   d.agentName(),
	})

	resourceManager.RegisterProvider("file", &resource.FileProvider{
		Prefix: folder,
	})
	results, err := resourceManager.Apply(agentResources)
	if err != nil {
		return fmt.Errorf("%w: %s", err, common.ProcessResourceApplyResults(results))
	}
	return nil
}

func hashFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return []byte{}, err
	}
	dockerfileMD5 := md5.Sum(data)
	return dockerfileMD5[:], nil
}

// ExitCode returns true if the agent is exited and its exit code.
func (s *dockerComposeDeployedAgent) ExitCode(ctx context.Context) (bool, int, error) {
	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return false, -1, fmt.Errorf("could not create Docker Compose project for agent: %w", err)
	}

	opts := compose.CommandOptions{Env: s.env}

	return p.ServiceExitCode(ctx, s.agentInfo.Name, opts)
}

// Logs returns the logs from the agent starting at the given time
func (s *dockerComposeDeployedAgent) Logs(ctx context.Context, t time.Time) ([]byte, error) {
	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for agent: %w", err)
	}

	opts := compose.CommandOptions{Env: s.env}

	return p.Logs(ctx, opts)
}

// TearDown tears down the agent.
func (s *dockerComposeDeployedAgent) TearDown(ctx context.Context) error {
	logger.Debugf("tearing down agent using Docker Compose runner")
	defer func() {
		// Remove the service logs dir for this agent
		if err := os.RemoveAll(s.agentInfo.Logs.Folder.Local); err != nil {
			logger.Errorf("could not remove the agent logs (path: %s): %v", s.agentInfo.Logs.Folder.Local, err)
		}

		// Remove the configuration dir for this agent (e.g. compose scenario files)
		if err := os.RemoveAll(s.configDir); err != nil {
			logger.Errorf("could not remove the agent configuration directory (path: %s) %v", s.configDir, err)
		}
	}()

	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{Env: s.env}
	processAgentContainerLogs(ctx, s.workDir, p, opts, s.agentInfo.Name)

	if err := p.Down(ctx, compose.CommandOptions{
		Env:       opts.Env,
		ExtraArgs: []string{"--volumes"}, // Remove associated volumes.
	}); err != nil {
		return fmt.Errorf("could not shut down agent using Docker Compose: %w", err)
	}
	return nil
}

// Info returns the current context for the agent.
func (s *dockerComposeDeployedAgent) Info() AgentInfo {
	return s.agentInfo
}

// SetInfo sets the current context for the agent.
func (s *dockerComposeDeployedAgent) SetInfo(info AgentInfo) {
	s.agentInfo = info
}
