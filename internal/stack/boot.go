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
	"github.com/elastic/elastic-package/internal/profile"
)

// baseComposeProjectName is the base name of the Docker Compose project used to boot up
// Elastic Stack containers.
const baseComposeProjectName = "elastic-package-stack"

// DockerComposeProjectName returns the docker compose project name for a given profile.
func DockerComposeProjectName(profile *profile.Profile) string {
	if profile.ProfileName == "default" {
		return baseComposeProjectName
	}
	return baseComposeProjectName + "-" + profile.ProfileName
}

// BootUp function boots up the Elastic stack.
func BootUp(options Options) error {
	// Print information before starting the stack, for cases where
	// this is executed in the foreground, without daemon mode.
	config := Config{
		Provider:              ProviderCompose,
		ElasticsearchHost:     "https://127.0.0.1:9200",
		ElasticsearchUsername: elasticsearchUsername,
		ElasticsearchPassword: elasticsearchPassword,
		KibanaHost:            "https://127.0.0.1:5601",
		CACertFile:            options.Profile.Path(CACertificateFile),
	}
	printUserConfig(options.Printer, config)

	buildPackagesPath, found, err := builder.FindBuildPackagesDirectory()
	if err != nil {
		return fmt.Errorf("finding build packages directory failed: %w", err)
	}

	stackPackagesDir, err := locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("locating stack packages directory failed: %w", err)
	}

	err = files.ClearDir(stackPackagesDir.PackagesDir())
	if err != nil {
		return fmt.Errorf("clearing package contents failed: %w", err)
	}

	if found {
		fmt.Printf("Custom build packages directory found: %s\n", buildPackagesPath)
		err = copyUniquePackages(buildPackagesPath, stackPackagesDir.PackagesDir())
		if err != nil {
			return fmt.Errorf("copying package contents failed: %w", err)
		}
	}

	options.Printer.Println("Local package-registry will serve packages from these sources:")
	options.Printer.Println("- Proxy to https://epr.elastic.co")

	if found {
		options.Printer.Printf("- Local directory %s\n", buildPackagesPath)
	}

	err = applyResources(options.Profile, options.StackVersion)
	if err != nil {
		return fmt.Errorf("creating stack files failed: %w", err)
	}

	err = dockerComposeBuild(options)
	if err != nil {
		return fmt.Errorf("building docker images failed: %w", err)
	}

	err = dockerComposeUp(options)
	if err != nil {
		// At least starting on 8.6.0, fleet-server may be reconfigured or
		// restarted after being healthy. If elastic-agent tries to enroll at
		// this moment, it fails inmediately, stopping and making `docker-compose up`
		// to fail too.
		// As a workaround, try to give another chance to docker-compose if only
		// elastic-agent failed.
		if onlyElasticAgentFailed(options) {
			fmt.Println("Elastic Agent failed to start, trying again.")
			err = dockerComposeUp(options)
		}
		if err != nil {
			return fmt.Errorf("running docker-compose failed: %w", err)
		}
	}

	err = storeConfig(options.Profile, config)
	if err != nil {
		return fmt.Errorf("failed to store config: %w", err)
	}

	return nil
}

func onlyElasticAgentFailed(options Options) bool {
	status, err := Status(options)
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
		return fmt.Errorf("stopping docker containers failed: %w", err)
	}
	return nil
}

func copyUniquePackages(sourcePath, destinationPath string) error {
	var skippedDirs []string

	dirEntries, err := os.ReadDir(sourcePath)
	if err != nil {
		return fmt.Errorf("can't read source dir (sourcePath: %s): %w", sourcePath, err)
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
