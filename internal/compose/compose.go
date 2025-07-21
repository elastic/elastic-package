// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package compose

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	// waitForHealthyTimeout is the maximum duration for WaitForHealthy().
	waitForHealthyTimeout = 10 * time.Minute
	// waitForHealthyInterval is the check interval for WaitForHealthy().
	waitForHealthyInterval = 1 * time.Second
)

var (
	EnableComposeStandaloneEnv     = environment.WithElasticPackagePrefix("COMPOSE_ENABLE_STANDALONE")
	DisableVerboseOutputComposeEnv = environment.WithElasticPackagePrefix("COMPOSE_DISABLE_VERBOSE_OUTPUT")
)

const (
	defaultComposeProgressOutput = "plain"
)

// Project represents a Docker Compose project.
type Project struct {
	name             string
	composeFilePaths []string

	dockerComposeStandalone        bool
	disableANSI                    bool
	disablePullProgressInformation bool
	progressOutput                 string
	composeVersion                 *semver.Version
}

// Config represents a Docker Compose configuration file.
type Config struct {
	Services map[string]service
}

type service struct {
	Ports       []portMapping
	Environment map[string]string
}

type portMapping struct {
	ExternalIP   string
	ExternalPort int
	InternalPort int
	Protocol     string
}

type intOrStringYaml int

func (p *intOrStringYaml) UnmarshalYAML(node *yaml.Node) error {
	var s string
	err := node.Decode(&s)
	if err == nil {
		i, err := strconv.Atoi(s)
		*p = intOrStringYaml(i)
		return err
	}

	return node.Decode(p)
}

// UnmarshalYAML unmarshals a Docker Compose port mapping in YAML to
// a portMapping.
func (p *portMapping) UnmarshalYAML(node *yaml.Node) error {
	// Depending on how the port mapping is specified in the Docker Compose
	// configuration file, sometimes a map is returned and other times a
	// string is returned. Here we first check if a map was returned.
	if node.Kind == yaml.MappingNode {
		b, err := yaml.Marshal(node)
		if err != nil {
			return fmt.Errorf("could not re-encode YAML map node to YAML: %w", err)
		}

		var s struct {
			HostIP    string          `yaml:"host_ip"`
			Target    intOrStringYaml // Docker compose v2 can define ports as strings.
			Published intOrStringYaml // Docker compose v2 can define ports as strings.
			Protocol  string
		}

		if err := yaml.Unmarshal(b, &s); err != nil {
			return fmt.Errorf("could not unmarshal YAML map node: %w", err)
		}

		p.InternalPort = int(s.Target)
		p.ExternalPort = int(s.Published)
		p.Protocol = s.Protocol
		p.ExternalIP = s.HostIP
		return nil
	}

	var str string
	if err := node.Decode(&str); err != nil {
		return err
	}

	// First, parse out the protocol.
	mapping, protocol, found := strings.Cut(str, "/")
	if !found {
		return errors.New("could not find protocol in port mapping")
	}
	p.Protocol = protocol

	// Now, try to parse out external host, external IP, and internal port.
	parts := strings.Split(mapping, ":")
	var externalIP, internalPortStr, externalPortStr string
	switch len(parts) {
	case 1:
		// All we have is an internal port.
		internalPortStr = parts[0]
	case 3:
		// We have an external IP, external port, and an internal port.
		externalIP = parts[0]
		externalPortStr = parts[1]
		internalPortStr = parts[2]
	default:
		return errors.New("could not parse port mapping")
	}

	internalPort, err := strconv.Atoi(internalPortStr)
	if err != nil {
		return fmt.Errorf("error parsing internal port as integer: %w", err)
	}
	p.InternalPort = internalPort

	if externalPortStr != "" {
		externalPort, err := strconv.Atoi(externalPortStr)
		if err != nil {
			return fmt.Errorf("error parsing external port as integer: %w", err)
		}
		p.ExternalPort = externalPort
	}

	p.ExternalIP = externalIP

	return nil
}

// CommandOptions encapsulates the environment variables, extra arguments, and Docker Compose services
// that can be passed to each Docker Compose command.
type CommandOptions struct {
	Env       []string
	ExtraArgs []string
	Services  []string
}

// NewProject creates a new Docker Compose project given a sequence of Docker Compose configuration files.
func NewProject(name string, paths ...string) (*Project, error) {
	// TODO: a lot of the checks in NewProject don't need to happen any more, we might want to rethink how we do this.
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("could not find Docker Compose configuration file: %s: %w", path, err)
		}

		if info.IsDir() {
			return nil, fmt.Errorf("expected Docker Compose configuration file (%s) to be a file, not a folder", path)
		}
	}

	var c Project
	c.name = name
	c.composeFilePaths = paths

	v, ok := os.LookupEnv(EnableComposeStandaloneEnv)
	if ok && strings.ToLower(v) != "false" {
		c.dockerComposeStandalone = true
	} else {
		c.dockerComposeStandalone = c.dockerComposeStandaloneRequired()
	}

	// Passing a nil context here because we are on initialization.
	ver, err := c.dockerComposeVersion(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to determine Docker Compose version: %w", err)
	}
	if ver.Major() < 2 {
		return nil, fmt.Errorf("required Docker Compose v2, found %s", ver.String())
	}
	logger.Debugf("Determined Docker Compose version: %v", ver)

	v, ok = os.LookupEnv(DisableVerboseOutputComposeEnv)
	if ok && strings.ToLower(v) != "false" {
		if c.composeVersion.LessThan(semver.MustParse("2.19.0")) {
			c.disableANSI = true
		} else {
			// --ansi never looks is ignored by "docker compose" and latest versions of "docker-compose"
			// adding --progress plain is a similar result as --ansi never
			// if set to "--progress quiet", there is no output at all from docker compose commands
			c.progressOutput = defaultComposeProgressOutput
		}
		c.disablePullProgressInformation = true
	}

	return &c, nil
}

// Up brings up a Docker Compose project.
func (p *Project) Up(ctx context.Context, opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "up")
	if p.disablePullProgressInformation {
		args = append(args, "--quiet-pull")
	}
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return fmt.Errorf("running Docker Compose up command failed: %w", err)
	}

	return nil
}

// Stop stops a Docker Compose project.
func (p *Project) Stop(ctx context.Context, opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "stop")
	args = append(args, opts.ExtraArgs...)

	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return fmt.Errorf("running Docker Compose stop command failed: %w", err)
	}

	return nil
}

// Down tears down a Docker Compose project.
func (p *Project) Down(ctx context.Context, opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "down")
	args = append(args, opts.ExtraArgs...)

	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return fmt.Errorf("running Docker Compose down command failed: %w", err)
	}

	return nil
}

// Build builds a Docker Compose project.
func (p *Project) Build(ctx context.Context, opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "build")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return fmt.Errorf("running Docker Compose build command failed: %w", err)
	}

	return nil
}

// Kill sends a signal to a service container.
func (p *Project) Kill(ctx context.Context, opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "kill")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return fmt.Errorf("running Docker Compose kill command failed: %w", err)
	}

	return nil
}

// Config returns the combined configuration for a Docker Compose project.
func (p *Project) Config(ctx context.Context, opts CommandOptions) (*Config, error) {
	args := p.baseArgs()
	args = append(args, "config")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	var b bytes.Buffer
	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env, stdout: &b}); err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(b.Bytes(), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Pull pulls down images for a Docker Compose project.
func (p *Project) Pull(ctx context.Context, opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "pull")
	if p.disablePullProgressInformation {
		args = append(args, "--quiet")
	}
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return fmt.Errorf("running Docker Compose pull command failed: %w", err)
	}

	return nil
}

// Logs returns service logs for the selected service in the Docker Compose project.
func (p *Project) Logs(ctx context.Context, opts CommandOptions) ([]byte, error) {
	args := p.baseArgs()
	args = append(args, "logs")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	var b bytes.Buffer
	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env, stdout: &b}); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// WaitForHealthy method waits until all containers are healthy.
func (p *Project) WaitForHealthy(ctx context.Context, opts CommandOptions) error {
	// Read container IDs
	args := p.baseArgs()
	args = append(args, "ps", "-a", "--format", "{{.ID}}")

	var b bytes.Buffer
	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env, stdout: &b}); err != nil {
		return err
	}

	ctx, stop := context.WithTimeout(ctx, waitForHealthyTimeout)
	defer stop()

	containerIDs := strings.Fields(b.String())
	logger.Debugf("Wait for healthy containers: %s", strings.Join(containerIDs, ","))
	for {
		// NOTE: healthy must be reinitialized at each iteration
		healthy := true

		descriptions, err := docker.InspectContainers(containerIDs...)
		if err != nil {
			return err
		}

		for _, d := range descriptions {
			dockerID := fmt.Sprintf("%.*s", 12, d.ID) // Ensure it is always 12 characters long
			switch {
			// No healthcheck defined for service
			case d.State.Status == "running" && d.State.Health == nil:
				logger.Debugf("Container %s (%s) status: %s (no health status)", d.Config.Labels.ComposeService, dockerID, d.State.Status)
				// Service is up and running and it's healthy
			case d.State.Status == "running" && d.State.Health.Status == "healthy":
				logger.Debugf("Container %s (%s) status: %s (health: %s)", d.Config.Labels.ComposeService, dockerID, d.State.Status, d.State.Health.Status)
				// Container started and finished with exit code 0
			case d.State.Status == "exited" && d.State.ExitCode == 0:
				logger.Debugf("Container %s (%s) status: %s (exit code: %d)", d.Config.Labels.ComposeService, dockerID, d.State.Status, d.State.ExitCode)
				// Container exited with code > 0
			case d.State.Status == "exited" && d.State.ExitCode > 0:
				logger.Debugf("Container %s (%s) status: %s (exit code: %d)", d.Config.Labels.ComposeService, dockerID, d.State.Status, d.State.ExitCode)
				return fmt.Errorf("container (ID: %s) exited with code %d", dockerID, d.State.ExitCode)
			// Any different status is considered unhealthy
			default:
				logger.Debugf("Container %s (%s) status: unhealthy", d.Config.Labels.ComposeService, dockerID)
				healthy = false
			}
		}

		// end loop before timeout if healthy
		if healthy {
			break
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return errors.New("timeout waiting for healthy container")
			}
			return ctx.Err()
		// NOTE: using after does not guarantee interval but it's ok for this use case
		case <-time.After(waitForHealthyInterval):
		}
	}

	return nil
}

// ServiceExitCode returns true if the specified service is exited with an error.
func (p *Project) ServiceExitCode(ctx context.Context, service string, opts CommandOptions) (bool, int, error) {
	// Read container IDs
	args := p.baseArgs()
	args = append(args, "ps", "-a", "-q", service)

	var b bytes.Buffer
	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, env: opts.Env, stdout: &b}); err != nil {
		return false, -1, err
	}

	containerIDs := strings.Fields(b.String())
	if len(containerIDs) != 1 {
		return false, -1, fmt.Errorf("expected to find one service container named: %s, found: %d", service, len(containerIDs))
	}
	containerID := containerIDs[0]

	containerDescriptions, err := docker.InspectContainers(containerID)
	if err != nil {
		return false, -1, err
	}
	if len(containerDescriptions) != 1 {
		return false, -1, fmt.Errorf("expected to get one service status, found: %d", len(containerIDs))
	}
	containerDescription := containerDescriptions[0]

	// Container exited with code > 0
	if containerDescription.State.Status == "exited" {
		return true, containerDescription.State.ExitCode, nil
	}

	return false, -1, nil
}

func (p *Project) baseArgs() []string {
	var args []string
	for _, path := range p.composeFilePaths {
		args = append(args, "-f", path)
	}

	if p.disableANSI {
		args = append(args, "--ansi", "never")
	}

	if p.progressOutput != "" {
		args = append(args, "--progress", p.progressOutput)
	}

	args = append(args, "-p", p.name)
	return args
}

type dockerComposeOptions struct {
	args   []string
	env    []string
	stdout io.Writer
}

const daemonResponse = `Error response from daemon:`

// This regexp must match prefixes like WARN[0000], which may include escape sequences for colored letters
// or structured logs, starting with key=value pairs.
var composeLoggerPrefix = regexp.MustCompile(`^[^\s]+\[[0-9]+\]`)

func cleanComposeError(msg string) string {
	// If there is a daemon response, just return it.
	if i := strings.Index(msg, daemonResponse); i >= 0 {
		return strings.TrimSpace(msg[i+len(daemonResponse):])
	}

	// Filter out lines coming from the docker compose structured logger.
	var cleanError strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(msg))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if composeLoggerPrefix.MatchString(line) {
			continue
		}
		fmt.Fprintln(&cleanError, line)
	}

	return strings.TrimSpace(cleanError.String())
}

func (p *Project) dockerComposeBaseCommand() (name string, args []string) {
	if p.dockerComposeStandalone {
		return "docker-compose", nil
	}
	return "docker", []string{"compose"}
}

func (p *Project) dockerComposeStandaloneRequired() bool {
	output, err := exec.Command("docker", "compose", "version", "--short").CombinedOutput()
	if err == nil {
		return false
	} else {
		logger.Debugf("docker compose subcommand failed: %v: %s", err, output)
	}

	return true
}

func (p *Project) dockerComposeVersion(ctx context.Context) (*semver.Version, error) {
	var b bytes.Buffer

	args := []string{
		"version",
		"--short",
	}
	if err := p.runDockerComposeCmd(ctx, dockerComposeOptions{args: args, stdout: &b}); err != nil {
		return nil, fmt.Errorf("running Docker Compose version command failed: %w", err)
	}
	dcVersion := b.String()
	ver, err := semver.NewVersion(strings.TrimSpace(dcVersion))
	if err != nil {
		return nil, fmt.Errorf("docker compose version is not a valid semver (value: %s): %w", dcVersion, err)
	}
	p.composeVersion = ver
	return ver, nil
}

// ContainerName method the container name for the service.
func (p *Project) ContainerName(serviceName string) string {
	return fmt.Sprintf("%s-%s-1", p.name, serviceName)
}
