// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"fmt"

	"github.com/elastic/elastic-package/internal/docker"
)

// Update pulls down the most recent versions of the Docker images.
func Update(ctx context.Context, options Options) error {
	err := applyResources(options.Profile, options.StackVersion)
	if err != nil {
		return fmt.Errorf("creating stack files failed: %w", err)
	}

	d := docker.NewDocker(docker.WithLogger(options.Logger))
	err = d.Pull(PackageRegistryBaseImage)
	if err != nil {
		return fmt.Errorf("pulling package-registry docker image failed: %w", err)
	}
	dockerCompose := newDockerCompose(dockerComposeOptions{
		Logger:       options.Logger,
		StackVersion: options.StackVersion,
		Profile:      options.Profile,
		Services:     options.Services,
	})

	err = dockerCompose.Pull(ctx)
	if err != nil {
		return fmt.Errorf("pulling docker images failed: %w", err)
	}
	return nil
}
