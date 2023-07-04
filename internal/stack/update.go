// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/docker"
)

// Update pulls down the most recent versions of the Docker images.
func Update(options Options) error {
	err := applyResources(options.Profile, options.StackVersion)
	if err != nil {
		return fmt.Errorf("creating stack files failed: %w", err)
	}

	err = docker.Pull(PackageRegistryBaseImage)
	if err != nil {
		return fmt.Errorf("pulling package-registry docker image failed: %w", err)
	}

	err = dockerComposePull(options)
	if err != nil {
		return fmt.Errorf("updating docker images failed: %w", err)
	}
	return nil
}
