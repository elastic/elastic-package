// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/files"
)

// DockerComposeProjectName is the name of the Docker Compose project used to boot up
// Elastic Stack containers.
const DockerComposeProjectName = "elastic-package-stack"

// BootUp function boots up the Elastic stack.
func BootUp(options Options) error {
	buildPackagesPath, found, err := builder.FindBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "finding build packages directory failed")
	}

	stackPackagesDir, err := locations.NewLocationManager()
	if err != nil {
		return errors.Wrap(err, "locating stack packages directory failed")
	}

	err = files.ClearDir(stackPackagesDir.PackagesDir())
	if err != nil {
		return errors.Wrap(err, "clearing package contents failed")
	}

	if found {
		fmt.Printf("Custom build packages directory found: %s\n", buildPackagesPath)
		err = copyUniquePackages(buildPackagesPath, stackPackagesDir.PackagesDir())
		if err != nil {
			return errors.Wrap(err, "copying package contents failed")
		}
	}

	fmt.Println("Packages from the following directories will be loaded into the package-registry:")
	fmt.Println("- built-in packages (package-storage:snapshot Docker image)")

	if found {
		fmt.Printf("- %s\n", buildPackagesPath)
	}

	err = dockerComposeBuild(options)
	if err != nil {
		return errors.Wrap(err, "building docker images failed")
	}

	err = dockerComposeUp(options)
	if err != nil {
		// At least starting on 8.6.0, fleet-server may be reconfigured or
		// restarted after being healthy. If elastic-agent tries to enroll at
		// this moment, it fails inmediately, stopping and making `docker-compose up`
		// to fail too.
		// As a workaround, try to give another chance to docker-compose if only
		// elastic-agent failed.
		if onlyElasticAgentFailed() {
			fmt.Println("Elastic Agent failed to start, trying again.")
			err = dockerComposeUp(options)
		}
		return errors.Wrap(err, "running docker-compose failed")
	}

	return nil
}

func onlyElasticAgentFailed() bool {
	status, err := Status()
	if err != nil {
		fmt.Printf("Failed to check status of the stack after failure: %v\n", err)
		return false
	}

	for _, service := range status {
		if strings.Contains(service.Name, "elastic-agent") {
			continue
		}
		if !strings.HasPrefix(service.Status, "running") {
			return false
		}
	}

	return true
}

// TearDown function takes down the testing stack.
func TearDown(options Options) error {
	err := dockerComposeDown(options)
	if err != nil {
		return errors.Wrap(err, "stopping docker containers failed")
	}
	return nil
}

func copyUniquePackages(sourcePath, destinationPath string) error {
	var skippedDirs []string

	dirEntries, err := os.ReadDir(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "can't read source dir (sourcePath: %s)", sourcePath)
	}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}

		name, versionZip, found := stringsCut(entry.Name(), "-")
		if !found {
			continue
		}
		version, _, found := stringsCut(versionZip, ".zip")
		if !found {
			continue
		}
		skippedDirs = append(skippedDirs, filepath.Join(name, version))
	}
	return files.CopyWithSkipped(sourcePath, destinationPath, skippedDirs)
}

// stringsCut has been imported from Go source code.
// Link: https://github.com/golang/go/blob/master/src/strings/strings.go#L1187
// Once we bump up Go dependency, this will be replaced with runtime function.
func stringsCut(s, sep string) (before, after string, found bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}
