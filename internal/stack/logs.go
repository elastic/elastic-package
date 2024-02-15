// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
)

func dockerComposeLogs(ctx context.Context, serviceName string, profile *profile.Profile) ([]byte, error) {
	appConfig, err := install.Configuration()
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	snapshotFile := profile.Path(ProfileStackPath, SnapshotFile)

	p, err := compose.NewProject(DockerComposeProjectName(profile), snapshotFile)
	if err != nil {
		return nil, fmt.Errorf("could not create docker compose project: %w", err)
	}

	opts := compose.CommandOptions{
		Env: newEnvBuilder().
			withEnvs(appConfig.StackImageRefs(install.DefaultStackVersion).AsEnv()).
			withEnv(stackVariantAsEnv(install.DefaultStackVersion)).
			withEnvs(profile.ComposeEnvVars()).
			build(),
		Services: []string{serviceName},
	}

	out, err := p.Logs(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("running command failed: %w", err)
	}
	return out, nil
}

func copyDockerInternalLogs(serviceName, outputPath string, profile *profile.Profile) error {
	switch serviceName {
	case elasticAgentService, fleetServerService:
	default:
		return nil // we need to pull internal logs only from Elastic-Agent and Fleets Server container
	}

	p, err := compose.NewProject(DockerComposeProjectName(profile))
	if err != nil {
		return fmt.Errorf("could not create docker compose project: %w", err)
	}

	outputPath = filepath.Join(outputPath, serviceName+"-internal")
	serviceContainer := p.ContainerName(serviceName)
	err = docker.Copy(serviceContainer, "/usr/share/elastic-agent/state/data/logs/", outputPath)
	if err != nil {
		return fmt.Errorf("docker copy failed: %w", err)
	}
	return nil
}
