package compose

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

// Project represents a Docker Compose project.
type Project struct {
	name             string
	composeFilePaths []string
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

	if err := runDockerComposeCmd(args, opts.Env); err != nil {
		return errors.Wrap(err, "running Docker Compose up command failed")
	}

	return nil
}

// Down tears down a Docker Compose project.
func (p *Project) Down(opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "down")
	args = append(args, opts.ExtraArgs...)

	if err := runDockerComposeCmd(args, opts.Env); err != nil {
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

	if err := runDockerComposeCmd(args, opts.Env); err != nil {
		return errors.Wrap(err, "running Docker Compose build command failed")
	}

	return nil
}

// Pull pulls down images for a Docker Compose project.
func (p *Project) Pull(opts CommandOptions) error {
	args := p.baseArgs()
	args = append(args, "pull")
	args = append(args, opts.ExtraArgs...)
	args = append(args, opts.Services...)

	if err := runDockerComposeCmd(args, opts.Env); err != nil {
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

func runDockerComposeCmd(args, env []string) error {
	cmd := exec.Command("docker-compose", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}
