// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		return fmt.Errorf("finding build packages directory failed: %s", err)
	}

	stackPackagesDir, err := locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("locating stack packages directory failed: %s", err)
	}

	err = files.ClearDir(stackPackagesDir.PackagesDir())
	if err != nil {
		return fmt.Errorf("clearing package contents failed: %s", err)
	}

	if found {
		fmt.Printf("Custom build packages directory found: %s\n", buildPackagesPath)
		err = copyUniquePackages(buildPackagesPath, stackPackagesDir.PackagesDir())
		if err != nil {
			return fmt.Errorf("copying package contents failed: %s", err)
		}
	}

	fmt.Println("Packages from the following directories will be loaded into the package-registry:")
	fmt.Println("- built-in packages (package-storage:snapshot Docker image)")

	if found {
		fmt.Printf("- %s\n", buildPackagesPath)
	}

	err = dockerComposeBuild(options)
	if err != nil {
		return fmt.Errorf("building docker images failed: %s", err)
	}

	err = dockerComposeUp(options)
	if err != nil {
		return fmt.Errorf("running docker-compose failed: %s", err)
	}

	return nil
}

// TearDown function takes down the testing stack.
func TearDown(options Options) error {
	err := dockerComposeDown(options)
	if err != nil {
		return fmt.Errorf("stopping docker containers failed: %s", err)
	}
	return nil
}

func copyUniquePackages(sourcePath, destinationPath string) error {
	var skippedDirs []string

	dirEntries, err := os.ReadDir(sourcePath)
	if err != nil {
		return fmt.Errorf("can't read source dir (sourcePath: %s): %s", sourcePath, err)
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
