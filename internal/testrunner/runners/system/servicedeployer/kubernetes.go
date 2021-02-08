// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	kindControlPlaneContainerName = "kind-control-plane"

	kindContext = "kind-kind"
)

// KubernetesServiceDeployer is responsible for deploying resources in the Kubernetes cluster.
type KubernetesServiceDeployer struct {
	definitionsDir string
}

// NewKubernetesServiceDeployer function creates a new instance of KubernetesServiceDeployer.
func NewKubernetesServiceDeployer(definitionsDir string) (*KubernetesServiceDeployer, error) {
	return &KubernetesServiceDeployer{
		definitionsDir: definitionsDir,
	}, nil
}

// SetUp function links the kind container with elastic-package-stack network, installs Elastic-Agent and optionally
// custom YAML definitions.
func (ksd KubernetesServiceDeployer) SetUp(ctxt ServiceContext) (DeployedService, error) {
	err := verifyKindContext()
	if err != nil {
		return nil, errors.Wrap(err, "kind context vefication failed")
	}

	kindControlPlaneContainerID, err := findKindControlPlane()
	if err != nil {
		return nil, errors.Wrap(err, "can't find kind-control plane node")
	}

	err = connectControlPlaneToElasticStackNetwork(kindControlPlaneContainerID)
	if err != nil {
		return nil, errors.Wrap(err, "can't connect control plane to Elastic stack network")
	}

	err = installElasticAgentUsingKubernetesDefinition()
	if err != nil {
		return nil, errors.Wrap(err, "can't install Elastic-Agent in Kubernetes cluster")
	}

	// TODO install custom definitions
	// TODO Test execution: List cluster agent only
	// TODO uninstall custom definitions

	panic("implement me")
}

var _ ServiceDeployer = new(KubernetesServiceDeployer)

func verifyKindContext() error {
	logger.Debug("ensure that kind context is selected")

	cmd := exec.Command("kubectl", "config", "current-context")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrapf(err, "kubectl command failed")
	}
	currentContext := string(output)

	if currentContext != "kind-kind" {
		return fmt.Errorf("unexpected kubectl context selected (actual: %s, expected: %s)", currentContext, kindContext)
	}
	return nil
}

func findKindControlPlane() (string, error) {
	logger.Debugf("find \"%s\" container", kindControlPlaneContainerName)

	cmd := exec.Command("docker", "ps", "--filter", "name=kind-control-plane", "--format", "{{.ID}}")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "could not find \"%s\" container to the stack network (stderr=%q)", kindControlPlaneContainerName, errOutput.String())
	}
	containerIDs := bytes.Split(bytes.TrimSpace(output), []byte{'\n'})
	if len(containerIDs) != 1 {
		return "", fmt.Errorf("expected single %s container, make sure you have run \"kind create cluster\" and the %s container is present", kindControlPlaneContainerName, kindControlPlaneContainerName)
	}
	return string(containerIDs[0]), nil
}

func connectControlPlaneToElasticStackNetwork(controlPlaneContainerID string) error {
	stackNetwork := fmt.Sprintf("%s_default", stack.DockerComposeProjectName)
	logger.Debugf("attaching service container %s (ID: %s) to stack network %s", kindControlPlaneContainerName, controlPlaneContainerID, stackNetwork)

	cmd := exec.Command("docker", "network", "connect", stackNetwork, controlPlaneContainerID)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "could not attach \"%s\" container to the stack network (stderr=%q)", kindControlPlaneContainerName, errOutput.String())
	}
	return nil
}

func installElasticAgentUsingKubernetesDefinition() error {
	elasticAgentFile, err := install.KubernetesDeployerElasticAgentFile()
	if err != nil {
		return errors.Wrap(err, "can't locate Kubernetes file for Elastic Agent in ")
	}

	cmd := exec.Command("kubectl", "apply", "-f", elasticAgentFile)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "kubectl apply failed (stderr=%q)", errOutput.String())
	}
	return nil
}
