// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
)

func dockerComposeLogsSince(ctx context.Context, serviceName string, profile *profile.Profile, since time.Time) ([]byte, error) {
	appConfig, err := install.Configuration(install.WithStackVersion(install.DefaultStackVersion), install.WithAgentVersion(install.DefaultStackVersion))
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	composeFile := profile.Path(ProfileStackPath, ComposeFile)

	p, err := compose.NewProject(DockerComposeProjectName(profile), composeFile)
	if err != nil {
		return nil, fmt.Errorf("could not create docker compose project: %w", err)
	}

	opts := compose.CommandOptions{
		Env: newEnvBuilder().
			withEnvs(appConfig.StackImageRefs().AsEnv()).
			withEnv(stackVariantAsEnv(install.DefaultStackVersion)).
			withEnvs(profile.ComposeEnvVars()).
			build(),
		Services: []string{serviceName},
	}

	if !since.IsZero() {
		opts.ExtraArgs = append(opts.ExtraArgs, "--since", since.UTC().Format("2006-01-02T15:04:05Z"))
	}

	out, err := p.Logs(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("running command failed: %w", err)
	}
	return out, nil
}

func copyDockerInternalLogs(serviceName, outputPath string, profile *profile.Profile) (string, error) {
	p, err := compose.NewProject(DockerComposeProjectName(profile))
	if err != nil {
		return "", fmt.Errorf("could not create docker compose project: %w", err)
	}

	outputPath = filepath.Join(outputPath, serviceName+"-internal")
	serviceContainer := p.ContainerName(serviceName)
	err = docker.Copy(serviceContainer, "/usr/share/elastic-agent/state/data/logs/", outputPath)
	if err != nil {
		return "", fmt.Errorf("docker copy failed: %w", err)
	}
	return outputPath, nil
}
