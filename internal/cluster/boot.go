package cluster

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/magefile/mage/sh"
)

func BootUp() error {
	buildPackagesPath, found, err := findBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "finding build packages directory failed")
	}

	clusterPackagesDir, err := install.ClusterPackagesDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster packages directory failed")
	}

	err = clearPackageContents(clusterPackagesDir)
	if err != nil {
		return errors.Wrap(err, "clearing package contents failed")
	}

	if found {
		fmt.Printf("Custom build packages directory found: %s\n", buildPackagesPath)
		err = copyPackageContents(buildPackagesPath, clusterPackagesDir)
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
		return errors.Wrap(err, "running docker-compose failed")
	}
	return nil
}

func TearDown() error {
	err := dockerComposeDown()
	if err != nil {
		return errors.Wrap(err, "stopping docker containers failed")
	}
	return nil
}

func findBuildPackagesDirectory() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, "build", "integrations") // TODO add support for other repositories
		fileInfo, err := os.Stat(path)
		if err == nil && fileInfo.IsDir() {
			return path, true, nil
		}

		if dir == "/" {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", false, nil
}

func clearPackageContents(destinationPath string) error {
	err := os.RemoveAll(destinationPath)
	if err != nil {
		return errors.Wrapf(err, "removing directory failed (path: %s)", destinationPath)
	}

	err = os.MkdirAll(destinationPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", destinationPath)
	}
	return nil
}

func copyPackageContents(sourcePath, destinationPath string) error {
	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}

		if relativePath == "." {
			return nil
		}

		if info.IsDir() {
			return os.MkdirAll(filepath.Join(destinationPath, relativePath), 0755)
		}

		return sh.Copy(
			filepath.Join(destinationPath, relativePath),
			filepath.Join(sourcePath, relativePath))
	})
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

func dockerComposeUp() error {
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
