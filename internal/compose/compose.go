// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package compose

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/signal"
)

const (
	// waitForHealthyTimeout is the maximum duration for WaitForHealthy().
	waitForHealthyTimeout = 10 * time.Minute
	// waitForHealthyInterval is the check interval for WaitForHealthy().
	waitForHealthyInterval = 1 * time.Second
)

// Project represents a Docker Compose project.
type Project struct {
	name             string
	composeFilePaths []string

	dockerComposeV1 bool
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

// UnmarshalYAML unmarshals a Docker Compose port mapping in YAML to
// a portMapping.
func (p *portMapping) UnmarshalYAML(node *yaml.Node) error {
	// Depending on how the port mapping is specified in the Docker Compose
	// configuration file, sometimes a map is returned and other times a
	// string is returned. Here we first check if a map was returned.
	if node.Kind == yaml.MappingNode {
		b, err := yaml.Marshal(node)
		if err != nil {
			return errors.Wrap(err, "could not re-encode YAML map node to YAML")
		}

		var s struct {
			HostIP    string `yaml:"host_ip"`
			Target    int
			Published int
			Protocol  string
		}

		if err := yaml.Unmarshal(b, &s); err != nil {
			return errors.Wrap(err, "could not unmarshal YAML map node")
		}

		p.InternalPort = s.Target
		p.ExternalPort = s.Published
		p.Protocol = s.Protocol
		p.ExternalIP = s.HostIP
		return nil
	}

	var str string
	if err := node.Decode(&str); err != nil {
		return err
	}

	// First, parse out the protocol.
	parts := strings.Split(str, "/")
	p.Protocol = parts[1]

	// Now, try to parse out external host, external IP, and internal port.
	parts = strings.Split(parts[0], ":")
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
		return errors.Wrap(err, "error parsing internal port as integer")
	}
	p.InternalPort = internalPort

	if externalPortStr != "" {
		externalPort, err := strconv.Atoi(externalPortStr)
		if err != nil {
			return errors.Wrap(err, "error parsing external port as integer")
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
			return nil, errors.Wrapf(err, "could not find Docker Compose configuration file: %s", path)
		}

		if info.IsDir() {
			return nil, fmt.Errorf("expected Docker Compose configuration file (%s) to be a file, not a folder", path)
		}
	}

	var c Project
	c.name = name
	c.composeFilePaths = paths

	ver, err := c.dockerComposeVersion()
	if err != nil {
		logger.Errorf("Unable to determine Docker Compose version: %v. Defaulting to 1.x", err)
		c.dockerComposeV1 = true
		return &c, nil
	}
	if ver.Major() == 1 {
		logger.Debugf("Determined Docker Compose version: %v, the tool will use Compose V1", err)
		c.dockerComposeV1 = true
	}
	return &c, nil
}

// Up brings up a Docker Compose project.
func (p *Project) Up(opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "up")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	if err := p.runDockerComposeCmd(dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return errors.Wrap(err, "running Docker Compose up command failed")
	}

	return nil
}

// Down tears down a Docker Compose project.
func (p *Project) Down(opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "down")
	args = append(args, opts.ExtraArgs...)

	if err := p.runDockerComposeCmd(dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return errors.Wrap(err, "running Docker Compose down command failed")
	}

	return nil
}

// Build builds a Docker Compose project.
func (p *Project) Build(opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "build")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	if err := p.runDockerComposeCmd(dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return errors.Wrap(err, "running Docker Compose build command failed")
	}

	return nil
}

// Kill sends a signal to a service container.
func (p *Project) Kill(opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "kill")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	if err := p.runDockerComposeCmd(dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return errors.Wrap(err, "running Docker Compose kill command failed")
	}

	return nil
}

// Config returns the combined configuration for a Docker Compose project.
func (p *Project) Config(opts CommandOptions) (*Config, error) {
	args := p.baseArgs()
	args = append(args, "config")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	var b bytes.Buffer
	if err := p.runDockerComposeCmd(dockerComposeOptions{args: args, env: opts.Env, stdout: &b}); err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(b.Bytes(), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Pull pulls down images for a Docker Compose project.
func (p *Project) Pull(opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "pull")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	if err := p.runDockerComposeCmd(dockerComposeOptions{args: args, env: opts.Env}); err != nil {
		return errors.Wrap(err, "running Docker Compose pull command failed")
	}

	return nil
}

// Logs returns service logs for the selected service in the Docker Compose project.
func (p *Project) Logs(opts CommandOptions) ([]byte, error) {
	args := p.baseArgs()
	args = append(args, "logs")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	var b bytes.Buffer
	if err := p.runDockerComposeCmd(dockerComposeOptions{args: args, env: opts.Env, stdout: &b}); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// WaitForHealthy method waits until all containers are healthy.
func (p *Project) WaitForHealthy(opts CommandOptions) error {
	// Read container IDs
	args := p.baseArgs()
	args = append(args, "ps")
	args = append(args, "-q")

	var b bytes.Buffer
	if err := p.runDockerComposeCmd(dockerComposeOptions{args: args, env: opts.Env, stdout: &b}); err != nil {
		return err
	}

	startTime := time.Now()
	timeout := startTime.Add(waitForHealthyTimeout)

	containerIDs := strings.Split(strings.TrimSpace(b.String()), "\n")
	for {
		if time.Now().After(timeout) {
			return errors.New("timeout waiting for healthy container")
		}

		if signal.SIGINT() {
			return errors.New("SIGINT: cancel waiting for policy assigned")
		}

		// NOTE: healthy must be reinitialized at each iteration
		healthy := true

		logger.Debugf("Wait for healthy containers: %s", strings.Join(containerIDs, ","))
		descriptions, err := docker.InspectContainers(containerIDs...)
		if err != nil {
			return err
		}

		for _, containerDescription := range descriptions {
			logger.Debugf("Container status: %s", containerDescription.String())

			// No healthcheck defined for service
			if containerDescription.State.Status == "running" && containerDescription.State.Health == nil {
				continue
			}

			// Service is up and running and it's healthy
			if containerDescription.State.Status == "running" && containerDescription.State.Health.Status == "healthy" {
				continue
			}

			// Container started and finished with exit code 0
			if containerDescription.State.Status == "exited" && containerDescription.State.ExitCode == 0 {
				continue
			}

			// Container exited with code > 0
			if containerDescription.State.Status == "exited" && containerDescription.State.ExitCode > 0 {
				return fmt.Errorf("container (ID: %s) exited with code %d", containerDescription.ID, containerDescription.State.ExitCode)
			}

			// Any different status is considered unhealthy
			healthy = false
		}

		// end loop before timeout if healthy
		if healthy {
			break
		}

		// NOTE: using sleep does not guarantee interval but it's ok for this use case
		time.Sleep(waitForHealthyInterval)
	}

	return nil
}

func (p *Project) baseArgs() []string {
	var args []string
	for _, path := range p.composeFilePaths {
		args = append(args, "-f", path)
	}

	args = append(args, "-p", p.name)
	return args
}

type dockerComposeOptions struct {
	args   []string
	env    []string
	stdout io.Writer
}

func (p *Project) runDockerComposeCmd(opts dockerComposeOptions) error {
	cmd := exec.Command("docker-compose", opts.args...)
	cmd.Env = append(os.Environ(), opts.env...)

	if logger.IsDebugMode() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if opts.stdout != nil {
		cmd.Stdout = opts.stdout
	}

	logger.Debugf("running command: %s", cmd)
	return cmd.Run()
}

func (p *Project) dockerComposeVersion() (*semver.Version, error) {
	var b bytes.Buffer

	args := []string{
		"version",
		"--short",
	}
	if err := p.runDockerComposeCmd(dockerComposeOptions{args: args, stdout: &b}); err != nil {
		return nil, errors.Wrap(err, "running Docker Compose version command failed")
	}
	dcVersion := b.String()
	ver, err := semver.NewVersion(strings.Trim(dcVersion, "\n"))
	if err != nil {
		return nil, errors.Wrapf(err, "docker compose version is not a valid semver (value: %s)", dcVersion)
	}
	return ver, nil
}

// ContainerName method the container name for the service.
func (p *Project) ContainerName(serviceName string) string {
	if p.dockerComposeV1 {
		return fmt.Sprintf("%s_%s_1", p.name, serviceName)
	}
	return fmt.Sprintf("%s-%s-1", p.name, serviceName)
}
