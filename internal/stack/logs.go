// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
)

func dockerComposeLogs(serviceName string, snapshotFile string) ([]byte, error) {
	p, err := compose.NewProject(DockerComposeProjectName, snapshotFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not create docker compose project")
	}

	opts := compose.CommandOptions{
		Services: []string{serviceName},
	}

	out, err := p.Logs(opts)
	if err != nil {
		return nil, errors.Wrap(err, "running command failed")
	}
	return out, nil
}

func dockerInternalLogs(serviceName string) ([]byte, bool, error) {
	if serviceName != fleetServerService {
		return nil, false, nil // we need to pull internal logs only from the Fleet Server container
	}

	p, err := compose.NewProject(DockerComposeProjectName)
	if err != nil {
		return nil, false, errors.Wrap(err, "could not create docker compose project")
	}

	serviceContainer := p.ContainerName(serviceName)
	out, err := docker.Exec(serviceContainer, "sh", "-c", `find data/logs/default/fleet-server-json* -printf '%p\n' -exec cat {} \;`)
	if err != nil {
		return nil, false, errors.Wrap(err, "docker exec failed")
	}
	return out, true, nil
}
