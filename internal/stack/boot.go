package stack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
)

// BootOptions defines available image booting options.
type BootOptions struct {
	DaemonMode   bool
	StackVersion string

	Services []string
}

// BootUp method boots up the testing stack.
func BootUp(options BootOptions) error {
	buildPackagesPath, found, err := builder.FindBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "finding build packages directory failed")
	}

	stackPackagesDir, err := install.StackPackagesDir()
	if err != nil {
		return errors.Wrap(err, "locating stack packages directory failed")
	}

	err = files.ClearDir(stackPackagesDir)
	if err != nil {
		return errors.Wrap(err, "clearing package contents failed")
	}

	if found {
		fmt.Printf("Custom build packages directory found: %s\n", buildPackagesPath)
		err = files.CopyAll(buildPackagesPath, stackPackagesDir)
		if err != nil {
			return errors.Wrap(err, "copying package contents failed")
		}
	}

	err = dockerComposeBuild(options)
	if err != nil {
		return errors.Wrap(err, "building docker images failed")
	}

	err = dockerComposeDown()
	if err != nil {
		return errors.Wrap(err, "stopping docker containers failed")
	}

	err = dockerComposeUp(options)
	if err != nil {
		return errors.Wrap(err, "running docker-compose failed")
	}
	return nil
}

// TearDown method takes down the testing stack.
func TearDown() error {
	err := dockerComposeDown()
	if err != nil {
		return errors.Wrap(err, "stopping docker containers failed")
	}
	return nil
}

// Update pulls down the most recent versions of the Docker images
func Update(options BootOptions) error {
	err := dockerComposePull(options)
	if err != nil {
		return errors.Wrap(err, "updating docker images failed")
	}
	return nil
}

func dockerComposeBuild(options BootOptions) error {
	stackDir, err := install.StackDir()
	if err != nil {
		return errors.Wrap(err, "locating stack directory failed")
	}

	args := []string{
		"-f", filepath.Join(stackDir, "snapshot.yml"),
		"build",
	}

	services := withIsReadyServices(withDependentServices(options.Services))
	if len(services) > 0 {
		args = append(args, services...)
	}

	cmd := exec.Command("docker-compose", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("STACK_VERSION=%s", options.StackVersion))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposePull(options BootOptions) error {
	stackDir, err := install.StackDir()
	if err != nil {
		return errors.Wrap(err, "locating stack directory failed")
	}

	args := []string{
		"-f", filepath.Join(stackDir, "snapshot.yml"),
		"pull",
	}

	services := withIsReadyServices(withDependentServices(options.Services))
	if len(services) > 0 {
		args = append(args, services...)
	}

	cmd := exec.Command("docker-compose", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("STACK_VERSION=%s", options.StackVersion))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposeUp(options BootOptions) error {
	stackDir, err := install.StackDir()
	if err != nil {
		return errors.Wrap(err, "locating stack directory failed")
	}

	args := []string{
		"-f", filepath.Join(stackDir, "snapshot.yml"),
		"up",
	}

	if options.DaemonMode {
		args = append(args, "-d")
	}

	services := withIsReadyServices(withDependentServices(options.Services))
	if len(services) > 0 {
		args = append(args, services...)
	}

	if len(options.Services) > 0 {
		for _, aService := range options.Services {
			args = append(args, aService, fmt.Sprintf("%s_is_ready", aService))
		}
	}

	cmd := exec.Command("docker-compose", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("STACK_VERSION=%s", options.StackVersion))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposeDown() error {
	stackDir, err := install.StackDir()
	if err != nil {
		return errors.Wrap(err, "locating stack directory failed")
	}

	cmd := exec.Command("docker-compose",
		"-f", filepath.Join(stackDir, "snapshot.yml"),
		"--project-directory", stackDir,
		"down")
	// We set the STACK_VERSION env var here to avoid showing a warning to the user about
	// it not being set.
	cmd.Env = append(os.Environ(), fmt.Sprintf("STACK_VERSION=%s", DefaultVersion))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func withDependentServices(services []string) []string {
	for _, aService := range services {
		if aService == "kibana" {
			return []string{} // kibana service requires to load all other services
		}
	}
	return services
}

func withIsReadyServices(services []string) []string {
	if len(services) == 0 {
		return services // load all defined services
	}

	var allServices []string
	for _, aService := range services {
		allServices = append(allServices, aService, fmt.Sprintf("%s_is_ready", aService))
	}
	return allServices
}
