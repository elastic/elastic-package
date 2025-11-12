// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/profile"
)

type localServicesManager struct {
	profile *profile.Profile
}

func (m *localServicesManager) start(ctx context.Context, options Options, config Config) error {
	err := applyLocalResources(m.profile, options.StackVersion, config)
	if err != nil {
		return fmt.Errorf("could not initialize compose files for local services: %w", err)
	}

	project, err := m.composeProject()
	if err != nil {
		return fmt.Errorf("could not initialize local services compose project")
	}

	opts := compose.CommandOptions{
		ExtraArgs: []string{},
	}
	err = project.Build(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to build images for local services: %w", err)
	}

	if options.DaemonMode {
		opts.ExtraArgs = append(opts.ExtraArgs, "-d")
	}
	if err := project.Up(ctx, opts); err != nil {
		// At least starting on 8.6.0, fleet-server may be reconfigured or
		// restarted after being healthy. If elastic-agent tries to enroll at
		// this moment, it fails inmediately, stopping and making `docker-compose up`
		// to fail too.
		// As a workaround, try to give another chance to docker-compose if only
		// elastic-agent failed.
		if onlyElasticAgentFailed(ctx, options) && !errors.Is(err, context.Canceled) {
			fmt.Println("Elastic Agent failed to start, trying again.")
			if err := project.Up(ctx, opts); err != nil {
				return fmt.Errorf("failed to start local services: %w", err)
			}
		}
	}

	return nil
}

func (m *localServicesManager) destroy(ctx context.Context) error {
	project, err := m.composeProject()
	if err != nil {
		return fmt.Errorf("could not initialize local services compose project")
	}

	opts := compose.CommandOptions{
		// Remove associated volumes.
		ExtraArgs: []string{"--volumes", "--remove-orphans"},
	}
	err = project.Down(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to destroy local services: %w", err)
	}

	return nil
}

func (m *localServicesManager) status() ([]ServiceStatus, error) {
	var services []ServiceStatus
	serviceStatusFunc := func(description docker.ContainerDescription) error {
		service, err := newServiceStatus(&description)
		if err != nil {
			return err
		}
		services = append(services, *service)
		return nil
	}

	err := m.visitDescriptions(serviceStatusFunc)
	if err != nil {
		return nil, err
	}

	return services, nil
}

func (m *localServicesManager) serviceNames() ([]string, error) {
	services := []string{}
	serviceFunc := func(description docker.ContainerDescription) error {
		services = append(services, description.Config.Labels.ComposeService)
		return nil
	}

	err := m.visitDescriptions(serviceFunc)
	if err != nil {
		return nil, err
	}

	return services, nil
}

func (m *localServicesManager) visitDescriptions(serviceFunc func(docker.ContainerDescription) error) error {
	// query directly to docker to avoid load environment variables (e.g. STACK_VERSION_VARIANT) and profiles
	project := m.composeProjectName()
	containerIDs, err := docker.ContainerIDsWithLabel(projectLabelDockerCompose, project)
	if err != nil {
		return err
	}

	if len(containerIDs) == 0 {
		return nil
	}

	containerDescriptions, err := docker.InspectContainers(containerIDs...)
	if err != nil {
		return err
	}

	for _, containerDescription := range containerDescriptions {
		serviceName := containerDescription.Config.Labels.ComposeService
		if strings.HasSuffix(serviceName, readyServicesSuffix) {
			continue
		}
		err := serviceFunc(containerDescription)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *localServicesManager) composeProject() (*compose.Project, error) {
	composeFile := m.profile.Path(ProfileStackPath, ComposeFile)
	return compose.NewProject(m.composeProjectName(), composeFile)
}

func (m *localServicesManager) composeProjectName() string {
	return DockerComposeProjectName(m.profile)
}
