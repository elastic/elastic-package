// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/profile"
)

// EnsureStackNetworkUp function verifies if stack network is up and running.
func EnsureStackNetworkUp(profile *profile.Profile) error {
	_, err := docker.InspectNetwork(Network(profile))
	return errors.Wrap(err, "network not available")
}

// Network function returns the stack network name.
func Network(profile *profile.Profile) string {
	return fmt.Sprintf("%s_default", DockerComposeProjectName(profile))
}
