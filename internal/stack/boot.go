// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

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
		err = files.CopyAll(buildPackagesPath, stackPackagesDir.PackagesDir())
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
		return errors.Wrap(err, "running docker-compose failed")
	}
	return nil
}

// TearDown function takes down the testing stack.
func TearDown(options Options) error {
	err := dockerComposeDown(options)
	if err != nil {
		return errors.Wrap(err, "stopping docker containers failed")
	}
	return nil
}
