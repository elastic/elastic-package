package cluster

import (
	"fmt"
	"github.com/elastic/elastic-package/internal/files"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/install"
)

// BootUp method boots up the testing cluster.
func BootUp() error {
	buildPackagesPath, found, err := builder.FindBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "finding build packages directory failed")
	}

	clusterPackagesDir, err := install.ClusterPackagesDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster packages directory failed")
	}

	err = files.ClearDir(clusterPackagesDir)
	if err != nil {
		return errors.Wrap(err, "clearing package contents failed")
	}

	if found {
		fmt.Printf("Custom build packages directory found: %s\n", buildPackagesPath)
		err = files.CopyAll(buildPackagesPath, clusterPackagesDir)
		if err != nil {
			return errors.Wrap(err, "copying package contents failed")
		}
	}

	err = dockerComposeBuild()
	if err != nil {
		return errors.Wrap(err, "building docker images failed")
	}

	err = dockerComposeDown()
	if err != nil {
		return errors.Wrap(err, "stopping docker containers failed")
	}

	err = dockerComposeUpD()
	if err != nil {
		return errors.Wrap(err, "running docker-compose failed")
	}
	return nil
}

// Run method runs the cluster and shows the logs.
func Run() error {
	buildPackagesPath, found, err := builder.FindBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "finding build packages directory failed")
	}

	clusterPackagesDir, err := install.ClusterPackagesDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster packages directory failed")
	}

	err = files.ClearDir(clusterPackagesDir)
	if err != nil {
		return errors.Wrap(err, "clearing package contents failed")
	}

	if found {
		fmt.Printf("Custom build packages directory found: %s\n", buildPackagesPath)
		err = files.CopyAll(buildPackagesPath, clusterPackagesDir)
		if err != nil {
			return errors.Wrap(err, "copying package contents failed")
		}
	}

	err = dockerComposeBuild()
	if err != nil {
		return errors.Wrap(err, "building docker images failed")
	}

	err = dockerComposeDown()
	if err != nil {
		return errors.Wrap(err, "stopping docker containers failed")
	}

	err = dockerComposeUp()
	if err != nil {
		return errors.Wrap(err, "running docker containers failed")
	}
	return nil
}

// TearDown method takes down the testing cluster.
func TearDown() error {
	err := dockerComposeDown()
	if err != nil {
		return errors.Wrap(err, "stopping docker containers failed")
	}
	return nil
}

func dockerComposeBuild() error {
	clusterDir, err := install.ClusterDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster directory failed")
	}

	args := []string{
		"-f", filepath.Join(clusterDir, "snapshot.yml"),
		"build", "package-registry",
	}
	cmd := exec.Command("docker-compose", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposeUpD() error {
	clusterDir, err := install.ClusterDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster directory failed")
	}

	args := []string{
		"-f", filepath.Join(clusterDir, "snapshot.yml"),
		"up", "-d",
	}
	cmd := exec.Command("docker-compose", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposeUp() error {
	clusterDir, err := install.ClusterDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster directory failed")
	}

	args := []string{
		"-f", filepath.Join(clusterDir, "snapshot.yml"),
		"up", "--force-recreate", "--remove-orphans", "--build",
	}
	cmd := exec.Command("docker-compose", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposeDown() error {
	clusterDir, err := install.ClusterDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster directory failed")
	}

	cmd := exec.Command("docker-compose",
		"-f", filepath.Join(clusterDir, "snapshot.yml"),
		"--project-directory", clusterDir,
		"down")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}
