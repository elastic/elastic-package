package cluster

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/install"
)

const envFile = ".env"

func BootUp() error {
	buildPackagesPath, found, err := findBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "finding build packages directory failed")
	}

	var envFileContent string
	if found {
		fmt.Printf("Custom build packages directory found: %s\n", buildPackagesPath)
		envFileContent = fmt.Sprintf("PACKAGES_PATH=%s\n", buildPackagesPath)
	}
	err = writeEnvFile(envFileContent)
	if err != nil {
		return errors.Wrapf(err, "writing .env file failed (packagesPath: %s)", buildPackagesPath)
	}

	err = dockerComposeUp(found)
	if err != nil {
		return errors.Wrap(err, "running docker-compose failed")
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

func writeEnvFile(content string) error {
	clusterDir, err := install.ClusterDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster directory failed")
	}
	envFilePath := filepath.Join(clusterDir, envFile)
	err = ioutil.WriteFile(envFilePath, []byte(content), 0644)
	if err != nil {
		return errors.Wrapf(err, "writing file failed (path: %s)", envFilePath)
	}
	return nil
}

func dockerComposeUp(useCustomPackagesPath bool) error {
	clusterDir, err := install.ClusterDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster directory failed")
	}

	var args []string
	args = append(args, "-f", filepath.Join(clusterDir, "snapshot.yml"))

	if useCustomPackagesPath {
		args = append(args, "-f", filepath.Join(clusterDir, "package-registry-volume.yml"))
	}

	args = append(args, "--project-directory", clusterDir,
		"up", "-d")

	cmd := exec.Command("docker-compose", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}
