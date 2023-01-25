// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/profile"
)

// Update pulls down the most recent versions of the Docker images.
func Update(options Options) error {
	err := docker.Pull(profile.PackageRegistryBaseImage)
	if err != nil {
		return fmt.Errorf("pulling package-registry docker image failed: %s", err)
	}

	err = dockerComposePull(options)
	if err != nil {
		return fmt.Errorf("updating docker images failed: %s", err)
	}
	return nil
}
