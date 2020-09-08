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

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/logger"
)

// Project represents a Docker Compose project.
type Project struct {
	name             string
	composeFilePaths []string
}

// Config represents a Docker Compose configuration file.
type Config struct {
	Services map[string]service
}
type service struct {
	Ports []portMapping
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
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, errors.Wrapf(err, "could not find Docker Compose configuration file: %s", path)
		}

		if info.IsDir() {
			return nil, fmt.Errorf("expected Docker Compose configuration file (%s) to be a file, not a folder", path)
		}
	}

	c := Project{
		name,
		paths,
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
	if opts.stdout != nil {
		cmd.Stdout = opts.stdout
	}

	logger.Debugf("running command: %s", cmd)

	return cmd.Run()
}
