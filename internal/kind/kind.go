// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kind

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/kubectl"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

// ControlPlaneContainerName is the Docker container name of the kind control plane node.
const ControlPlaneContainerName = "kind-control-plane"

const kindContext = "kind-kind"

type Client struct {
	profile *profile.Profile
	logger  *slog.Logger
}

type KindOption func(k *Client)

func NewKindClient(profile *profile.Profile, opts ...KindOption) *Client {
	c := Client{
		profile: profile,
		logger:  logger.Logger,
	}

	for _, opt := range opts {
		opt(&c)
	}
	return &c
}

func WithLogger(log *slog.Logger) KindOption {
	return func(k *Client) {
		k.logger = log
	}
}

// VerifyContext function ensures that the kind context is selected.
func (k *Client) VerifyContext(ctx context.Context) error {
	k.logger.Debug("ensure that kind context is selected")

	kubectlClient := kubectl.NewKubectlClient(kubectl.WithLogger(k.logger))

	currentContext, err := kubectlClient.CurrentContext(ctx)
	if err != nil {
		return fmt.Errorf("can't read current Kubernetes context: %w", err)
	}
	if currentContext != kindContext {
		return fmt.Errorf("unexpected Kubernetes context selected (actual: %s, expected: %s)", currentContext, kindContext)
	}
	return nil
}

// ConnectToElasticStackNetwork function ensures that the control plane node is connected to the Elastic stack network.
func (k *Client) ConnectToElasticStackNetwork() error {
	containerID, err := k.controlPlaneContainerID()
	if err != nil {
		return fmt.Errorf("can't find kind-control plane node: %w", err)
	}

	stackNetwork := stack.Network(k.profile)
	k.logger.Debug("check network connectivity between service container and the stack network",
		slog.Group("container", slog.String("name", ControlPlaneContainerName), slog.String("container.id", containerID)),
		slog.String("stack.network", stackNetwork),
	)

	d := docker.NewDocker(docker.WithLogger(k.logger))
	networkDescriptions, err := d.InspectNetwork(stackNetwork)
	if err != nil {
		return fmt.Errorf("can't inspect network: %w", err)
	}
	if len(networkDescriptions) != 1 {
		return fmt.Errorf("expected single network description, got %d entries", len(networkDescriptions))
	}

	for _, c := range networkDescriptions[0].Containers {
		if c.Name == ControlPlaneContainerName {
			k.logger.Debug("container is already attached to the network",
				slog.String("container.name", ControlPlaneContainerName),
				slog.String("stack.network", stackNetwork),
			)
			return nil
		}
	}

	k.logger.Debug("attach container to stack network",
		slog.Group("container", slog.String("name", ControlPlaneContainerName), slog.String("container.id", containerID)),
		slog.String("stack.network", stackNetwork),
	)
	err = d.ConnectToNetwork(containerID, stackNetwork)
	if err != nil {
		return fmt.Errorf("can't connect to the Elastic stack network: %w", err)
	}
	return nil
}

func (k *Client) controlPlaneContainerID() (string, error) {
	k.logger.Debug("find container", slog.String("container", ControlPlaneContainerName))

	d := docker.NewDocker(docker.WithLogger(k.logger))
	containerID, err := d.ContainerID(ControlPlaneContainerName)
	if err != nil {
		return "", fmt.Errorf("can't find container ID, make sure you have run \"kind create cluster\": %w", err)
	}
	return containerID, nil
}
