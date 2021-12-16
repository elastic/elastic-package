// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"path/filepath"

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

func copyDockerInternalLogs(serviceName, outputPath string) error {
	switch serviceName {
	case elasticAgentService, fleetServerService:
	default:
		return nil // we need to pull internal logs only from Elastic-Agent and Fleets Server container
	}

	p, err := compose.NewProject(DockerComposeProjectName)
	if err != nil {
		return errors.Wrap(err, "could not create docker compose project")
	}

	outputPath = filepath.Join(outputPath, serviceName+"-internal")
	serviceContainer := p.ContainerName(serviceName)
	err = docker.Copy(serviceContainer, "/usr/share/elastic-agent/state/data/logs/default", outputPath)
	if err != nil {
		return errors.Wrap(err, "docker copy failed")
	}
	return nil
}
