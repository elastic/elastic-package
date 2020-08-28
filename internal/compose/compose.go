package compose

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

// Project represents a Docker Compose project.
type Project struct {
	name  string
	paths []string
}

// NewProject creates a new Docker Compose project given a sequence of Docker Compose configuration files.
func NewProject(name string, paths ...string) (*Project, error) {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil && os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "could not find docker-compose configuration file: %s", path)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "error finding docker-compose configuration file: %s", path)
		}

		if info.IsDir() {
			return nil, fmt.Errorf("expected docker-compose configuration file (%s) to be a file, not a folder", path)
		}
	}

	c := Project{
		name,
		paths,
	}

	return &c, nil
}

// Up brings up a Docker Compose project.
func (p *Project) Up(extraArgs, env []string, services ...string) error {
	args := p.baseArgs()
	args = append(args, "up")
	args = append(args, extraArgs...)
	args = append(args, services...)

	if err := runDockerComposeCmd(args, env); err != nil {
		return errors.Wrap(err, "running docker-compose up command failed")
	}

	return nil
}

// Down tears down a Docker Compose project.
func (p *Project) Down(extraArgs, env []string) error {
	args := p.baseArgs()
	args = append(args, "down")
	args = append(args, extraArgs...)

	if err := runDockerComposeCmd(args, env); err != nil {
		return errors.Wrap(err, "running docker-compose down command failed")
	}

	return nil
}

// Build builds a Docker Compose project.
func (p *Project) Build(extraArgs, env []string, services ...string) error {
	args := p.baseArgs()
	args = append(args, "build")
	args = append(args, extraArgs...)
	args = append(args, services...)

	if err := runDockerComposeCmd(args, env); err != nil {
		return errors.Wrap(err, "running docker-compose build command failed")
	}

	return nil
}

// Pull pulls down images for a Docker Compose project.
func (p *Project) Pull(extraArgs, env []string, services ...string) error {
	args := p.baseArgs()
	args = append(args, "pull")
	args = append(args, extraArgs...)
	args = append(args, services...)

	if err := runDockerComposeCmd(args, env); err != nil {
		return errors.Wrap(err, "running docker-compose pull command failed")
	}

	return nil
}

func (p *Project) baseArgs() []string {
	var args []string
	for _, path := range p.paths {
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
