// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kind

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/kubectl"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

// ControlPlaneContainerName is the Docker container name of the kind control plane node.
const ControlPlaneContainerName = "kind-control-plane"

const kindContext = "kind-kind"

// VerifyContext function ensures that the kind context is selected.
func VerifyContext() error {
	logger.Debug("ensure that kind context is selected")

	currentContext, err := kubectl.CurrentContext()
	if err != nil {
		return errors.Wrap(err, "can't read current Kubernetes context")
	}
	if currentContext != kindContext {
		return fmt.Errorf("unexpected Kubernetes context selected (actual: %s, expected: %s)", currentContext, kindContext)
	}
	return nil
}

// ConnectToElasticStackNetwork function ensures that the control plane node is connected to the Elastic stack network.
func ConnectToElasticStackNetwork() error {
	containerID, err := controlPlaneContainerID()
	if err != nil {
		return errors.Wrap(err, "can't find kind-control plane node")
	}

	stackNetwork := stack.Network()
	logger.Debugf("check network connectivity between service container %s (ID: %s) and the stack network %s", ControlPlaneContainerName, containerID, stackNetwork)

	networkDescriptions, err := docker.InspectNetwork(stackNetwork)
	if err != nil {
		return errors.Wrap(err, "can't inspect network")
	}
	if len(networkDescriptions) != 1 {
		return fmt.Errorf("expected single network description, got %d entries", len(networkDescriptions))
	}

	for _, c := range networkDescriptions[0].Containers {
		if c.Name == ControlPlaneContainerName {
			logger.Debugf("container %s is already attached to the %s network", ControlPlaneContainerName, stackNetwork)
			return nil
		}
	}

	logger.Debugf("attach %s container (ID: %s) to stack network %s", ControlPlaneContainerName, containerID, stackNetwork)
	err = docker.ConnectToNetwork(containerID, stackNetwork)
	if err != nil {
		return errors.Wrap(err, "can't connect to the Elastic stack network")
	}
	return nil
}

func controlPlaneContainerID() (string, error) {
	logger.Debugf("find \"%s\" container", ControlPlaneContainerName)

	containerID, err := docker.ContainerID(ControlPlaneContainerName)
	if err != nil {
		return "", errors.Wrap(err, "can't find container ID, make sure you have run \"kind create cluster\"")
	}
	return containerID, nil
}
