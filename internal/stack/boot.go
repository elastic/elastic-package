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

	err = applyResources(options.Profile, options.StackVersion)
	if err != nil {
		return errors.Wrap(err, "creating stack files failed")
	}

	err = dockerComposeBuild(options)
	if err != nil {
		return errors.Wrap(err, "building docker images failed")
	}

	err = dockerComposeUp(options)
	if err != nil {
		return errors.Wrap(err, "running docker-compose failed")
	}

	config := Config{
		Provider:              ProviderCompose,
		ElasticsearchHost:     "https://127.0.0.1:9200",
		ElasticsearchUsername: elasticsearchUsername,
		ElasticsearchPassword: elasticsearchPassword,
		KibanaHost:            "https://127.0.0.1:5601",
		CACertFile:            options.Profile.Path(profileStackPath, CACertificateFile),
	}
	err = storeConfig(options.Profile, config)
	if err != nil {
		return errors.Wrap(err, "failed to store config")
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
