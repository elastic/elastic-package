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

// BootUp method boots up the testing stack.
func BootUp(daemonMode bool, stackVersion string) error {
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

	err = dockerComposeBuild(stackVersion)
	if err != nil {
		return errors.Wrap(err, "building docker images failed")
	}

	err = dockerComposeDown()
	if err != nil {
		return errors.Wrap(err, "stopping docker containers failed")
	}

	err = dockerComposeUp(daemonMode, stackVersion)
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
func Update(stackVersion string) error {
	err := dockerComposePull(stackVersion)
	if err != nil {
		return errors.Wrap(err, "updating docker images failed")
	}
	return nil
}

func dockerComposeBuild(stackVersion string) error {
	stackDir, err := install.StackDir()
	if err != nil {
		return errors.Wrap(err, "locating stack directory failed")
	}

	args := []string{
		"-f", filepath.Join(stackDir, "snapshot.yml"),
		"build", "package-registry",
	}
	cmd := exec.Command("docker-compose", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("STACK_VERSION=%s", stackVersion))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposePull(stackVersion string) error {
	stackDir, err := install.StackDir()
	if err != nil {
		return errors.Wrap(err, "locating stack directory failed")
	}

	args := []string{
		"-f", filepath.Join(stackDir, "snapshot.yml"),
		"pull",
	}
	cmd := exec.Command("docker-compose", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("STACK_VERSION=%s", stackVersion))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposeUp(daemonMode bool, stackVersion string) error {
	stackDir, err := install.StackDir()
	if err != nil {
		return errors.Wrap(err, "locating stack directory failed")
	}

	args := []string{
		"-f", filepath.Join(stackDir, "snapshot.yml"),
		"up",
	}

	if daemonMode {
		args = append(args, "-d")
	}

	cmd := exec.Command("docker-compose", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("STACK_VERSION=%s", stackVersion))
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
