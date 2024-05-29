// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
)

func dockerComposeLogsSince(ctx context.Context, serviceName string, profile *profile.Profile, since time.Time, logger *slog.Logger) ([]byte, error) {
	appConfig, err := install.Configuration()
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	composeFile := profile.Path(ProfileStackPath, ComposeFile)

	p, err := compose.NewProject(compose.ProjectOptions{
		Name:   DockerComposeProjectName(profile),
		Paths:  []string{composeFile},
		Logger: logger,
	})
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

	if !since.IsZero() {
		opts.ExtraArgs = append(opts.ExtraArgs, "--since", since.UTC().Format("2006-01-02T15:04:05Z"))
	}

	out, err := p.Logs(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("running command failed: %w", err)
	}
	return out, nil
}

func copyDockerInternalLogs(serviceName, outputPath string, profile *profile.Profile, logger *slog.Logger) (string, error) {
	p, err := compose.NewProject(compose.ProjectOptions{
		Name:   DockerComposeProjectName(profile),
		Logger: logger,
	})
	if err != nil {
		return "", fmt.Errorf("could not create docker compose project: %w", err)
	}

	outputPath = filepath.Join(outputPath, serviceName+"-internal")
	serviceContainer := p.ContainerName(serviceName)

	d := docker.NewDocker(docker.WithLogger(logger))
	err = d.Copy(serviceContainer, "/usr/share/elastic-agent/state/data/logs/", outputPath)
	if err != nil {
		return "", fmt.Errorf("docker copy failed: %w", err)
	}
	return outputPath, nil
}
