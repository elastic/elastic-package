// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
)

type envBuilder struct {
	vars []string
}

func newEnvBuilder() *envBuilder {
	return new(envBuilder)
}

func (eb *envBuilder) withEnvs(envs []string) *envBuilder {
	eb.vars = append(eb.vars, envs...)
	return eb
}

func (eb *envBuilder) withEnv(env string) *envBuilder {
	eb.vars = append(eb.vars, env)
	return eb
}

func (eb *envBuilder) build() []string {
	return eb.vars
}

func dockerComposeBuild(options Options) error {
	c, err := compose.NewProject(DockerComposeProjectName, options.Profile.FetchPath(profile.SnapshotFile))
	if err != nil {
		return errors.Wrap(err, "could not create docker compose project")
	}

	appConfig, err := install.Configuration()
	if err != nil {
		return errors.Wrap(err, "can't read application configuration")
	}

	opts := compose.CommandOptions{
		Env: newEnvBuilder().
			withEnvs(appConfig.StackImageRefs(options.StackVersion).AsEnv()).
			withEnv(stackVariantAsEnv(options.StackVersion)).
			withEnvs(options.Profile.ComposeEnvVars()).
			build(),
		Services: withIsReadyServices(withDependentServices(options.Services)),
	}

	if err := c.Build(opts); err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposePull(options Options) error {
	c, err := compose.NewProject(DockerComposeProjectName, options.Profile.FetchPath(profile.SnapshotFile))
	if err != nil {
		return errors.Wrap(err, "could not create docker compose project")
	}

	appConfig, err := install.Configuration()
	if err != nil {
		return errors.Wrap(err, "can't read application configuration")
	}

	opts := compose.CommandOptions{
		Env: newEnvBuilder().
			withEnvs(appConfig.StackImageRefs(options.StackVersion).AsEnv()).
			withEnv(stackVariantAsEnv(options.StackVersion)).
			withEnvs(options.Profile.ComposeEnvVars()).
			build(),
		Services: withIsReadyServices(withDependentServices(options.Services)),
	}

	if err := c.Pull(opts); err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposeUp(options Options) error {
	c, err := compose.NewProject(DockerComposeProjectName, options.Profile.FetchPath(profile.SnapshotFile))
	if err != nil {
		return errors.Wrap(err, "could not create docker compose project")
	}

	var args []string
	if options.DaemonMode {
		args = append(args, "-d")
	}

	appConfig, err := install.Configuration()
	if err != nil {
		return errors.Wrap(err, "can't read application configuration")
	}

	opts := compose.CommandOptions{
		Env: newEnvBuilder().
			withEnvs(appConfig.StackImageRefs(options.StackVersion).AsEnv()).
			withEnv(stackVariantAsEnv(options.StackVersion)).
			withEnvs(options.Profile.ComposeEnvVars()).
			build(),
		ExtraArgs: args,
		Services:  withIsReadyServices(withDependentServices(options.Services)),
	}

	if err := c.Up(opts); err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func dockerComposeDown(options Options) error {
	c, err := compose.NewProject(DockerComposeProjectName, options.Profile.FetchPath(profile.SnapshotFile))
	if err != nil {
		return errors.Wrap(err, "could not create docker compose project")
	}

	appConfig, err := install.Configuration()
	if err != nil {
		return errors.Wrap(err, "can't read application configuration")
	}

	downOptions := compose.CommandOptions{
		Env: newEnvBuilder().
			withEnvs(appConfig.StackImageRefs(options.StackVersion).AsEnv()).
			withEnv(stackVariantAsEnv(options.StackVersion)).
			withEnvs(options.Profile.ComposeEnvVars()).
			build(),
		// Remove associated volumes.
		ExtraArgs: []string{"--volumes", "--remove-orphans"},
	}
	if err := c.Down(downOptions); err != nil {
		return errors.Wrap(err, "running command failed")
	}
	return nil
}

func withDependentServices(services []string) []string {
	for _, aService := range services {
		if aService == "elastic-agent" {
			return []string{} // elastic-agent service requires to load all other services
		}
	}
	return services
}

func withIsReadyServices(services []string) []string {
	if len(services) == 0 {
		return services // load all defined services
	}

	var allServices []string
	for _, aService := range services {
		allServices = append(allServices, aService, fmt.Sprintf("%s_is_ready", aService))
	}
	return allServices
}
