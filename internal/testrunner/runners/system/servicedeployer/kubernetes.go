// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

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

type kubernetesDeployedService struct {
	ctxt ServiceContext

	definitionsDir string
}

func (s kubernetesDeployedService) TearDown() error {
	logger.Debugf("uninstall custom Kubernetes definitions (directory: %s)", s.definitionsDir)

	definitionPaths, err := findKubernetesDefinitions(s.definitionsDir)
	if err != nil {
		return errors.Wrapf(err, "can't find Kubernetes definitions in given directory (path: %s)", s.definitionsDir)
	}

	if len(definitionPaths) == 0 {
		logger.Debugf("no custom definitions found (directory: %s). Nothing will be uninstalled.", s.definitionsDir)
		return nil
	}

	err = modifyKubernetesResources(false, definitionPaths...)
	if err != nil {
		return errors.Wrapf(err, "can't uninstall Kubernetes resources (path: %s)", s.definitionsDir)
	}
	return nil
}

func (s kubernetesDeployedService) Signal(signal string) error {
	return errors.New("signal is not supported")
}

func (s kubernetesDeployedService) Context() ServiceContext {
	return s.ctxt
}

func (s kubernetesDeployedService) SetContext(sc ServiceContext) error {
	s.ctxt = sc
	return nil
}

var _ DeployedService = new(kubernetesDeployedService)

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

	err = installElasticAgentInCluster()
	if err != nil {
		return nil, errors.Wrap(err, "can't install Elastic-Agent in the Kubernetes cluster")
	}

	err = ksd.installCustomDefinitions()
	if err != nil {
		return nil, errors.Wrap(err, "can't install custom definitions in the Kubernetes cluster")
	}

	ctxt.Name = kindControlPlaneContainerName
	ctxt.Hostname = kindControlPlaneContainerName
	ctxt.Agent.Host.NamePrefix = "kind-fleet-agent-"
	return &kubernetesDeployedService{
		ctxt:           ctxt,
		definitionsDir: ksd.definitionsDir,
	}, nil
}

func (ksd KubernetesServiceDeployer) installCustomDefinitions() error {
	logger.Debugf("install custom Kubernetes definitions (directory: %s)", ksd.definitionsDir)

	definitionPaths, err := findKubernetesDefinitions(ksd.definitionsDir)
	if err != nil {
		return errors.Wrapf(err, "can't find Kubernetes definitions in given directory (path: %s)", ksd.definitionsDir)
	}

	if len(definitionPaths) == 0 {
		logger.Debugf("no custom definitions found (directory: %s). Nothing else will be installed.", ksd.definitionsDir)
		return nil
	}

	err = modifyKubernetesResources(false, definitionPaths...)
	if err != nil {
		return errors.Wrap(err, "can't install custom definitions")
	}
	return nil
}

var _ ServiceDeployer = new(KubernetesServiceDeployer)

func findKubernetesDefinitions(definitionsDir string) ([]string, error) {
	fileInfos, err := ioutil.ReadDir(definitionsDir)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read definitions directory (path: %s)", definitionsDir)
	}

	var definitionPaths []string
	for _, fileInfo := range fileInfos {
		if strings.HasSuffix(fileInfo.Name(), ".yaml") {
			definitionPaths = append(definitionPaths, filepath.Join(definitionsDir, fileInfo.Name()))
		}
	}
	return definitionPaths, nil
}

func verifyKindContext() error {
	logger.Debug("ensure that kind context is selected")

	cmd := exec.Command("kubectl", "config", "current-context")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrapf(err, "kubectl command failed (stderr=%q)", errOutput.String())
	}
	currentContext := string(bytes.TrimSpace(output))

	if currentContext != kindContext {
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

	logger.Debugf("check network connectivity between service container %s (ID: %s) and the stack network %s", kindControlPlaneContainerName, controlPlaneContainerID, stackNetwork)
	cmd := exec.Command("docker", "network", "inspect", stackNetwork)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrapf(err, "could not inspect the stack network (stderr=%q)", errOutput.String())
	}

	var networkDescriptions []struct {
		Containers map[string]struct {
			Name string
		}
	}
	err = json.Unmarshal(output, &networkDescriptions)
	if err != nil {
		return errors.Wrapf(err, "can't unmarshal network inspect for %s (stderr=%q)", stackNetwork, errOutput.String())
	}

	if len(networkDescriptions) != 1 {
		return fmt.Errorf("expect single network inspect record, got %d entries", len(networkDescriptions))
	}

	for _, c := range networkDescriptions[0].Containers {
		if c.Name == kindControlPlaneContainerName {
			logger.Debugf("container %s is already attached to the %s network", kindControlPlaneContainerName, stackNetwork)
			return nil
		}
	}

	logger.Debugf("attach service container %s (ID: %s) to stack network %s", kindControlPlaneContainerName, controlPlaneContainerID, stackNetwork)
	cmd = exec.Command("docker", "network", "connect", stackNetwork, controlPlaneContainerID)
	errOutput = new(bytes.Buffer)
	cmd.Stderr = errOutput
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "could not attach \"%s\" container to the stack network (stderr=%q)", kindControlPlaneContainerName, errOutput.String())
	}
	return nil
}

func installElasticAgentInCluster() error {
	logger.Debug("install Elastic Agent in the Kubernetes cluster")

	elasticAgentFile, err := install.KubernetesDeployerElasticAgentFile()
	if err != nil {
		return errors.Wrap(err, "can't locate Kubernetes file for Elastic Agent in ")
	}

	err = modifyKubernetesResources(true, elasticAgentFile)
	if err != nil {
		return errors.Wrap(err, "can't install Elastic-Agent in Kubernetes cluster")
	}
	return nil
}

func modifyKubernetesResources(apply bool, definitionPaths ...string) error {
	var args []string
	if apply {
		args = append(args, "apply")
	} else {
		args = append(args, "delete")
	}

	for _, definitionPath := range definitionPaths {
		args = append(args, "-f")
		args = append(args, definitionPath)
	}

	cmd := exec.Command("kubectl", args...)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "kubectl apply failed (stderr=%q)", errOutput.String())
	}
	return nil
}
