// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/servicedeployer"
)

const (
	TypeTest  = "test"
	TypeBench = "bench"
)

// FactoryOptions defines options used to create an instance of a service deployer.
type FactoryOptions struct {
	Profile *profile.Profile

	PackageRoot          string
	DataStreamRoot       string
	DevDeployDir         string
	Type                 string
	StackVersion         string
	OverrideAgentVersion string
	PolicyName           string

	DeployerName string

	PackageName string
	DataStream  string

	RunTearDown  bool
	RunTestsOnly bool
	RunSetup     bool
}

// Factory chooses the appropriate service runner for the given data stream, depending
// on service configuration files defined in the package or data stream.
func Factory(options FactoryOptions) (AgentDeployer, error) {
	agentDeployerName, err := selectAgentDeployerType(options)
	if err != nil {
		return nil, fmt.Errorf("failed to select agent deployer type: %w", err)
	}

	switch agentDeployerName {
	case "default":
		if options.Type != TypeTest {
			return nil, fmt.Errorf("agent deployer is not supported for type %s", options.Type)
		}
		opts := DockerComposeAgentDeployerOptions{
			Profile:              options.Profile,
			StackVersion:         options.StackVersion,
			PackageName:          options.PackageName,
			PolicyName:           options.PolicyName,
			DataStream:           options.DataStream,
			RunTearDown:          options.RunTearDown,
			RunTestsOnly:         options.RunTestsOnly,
			OverrideAgentVersion: options.OverrideAgentVersion,
		}
		return NewCustomAgentDeployer(opts)
	case "agent":
		// FIXME: should this be just carried out by service deployer?
		// FIXME: this docker-compose scenario contains both agent and service
		return nil, nil
	case "k8s":
		opts := KubernetesAgentDeployerOptions{
			Profile:              options.Profile,
			StackVersion:         options.StackVersion,
			OverrideAgentVersion: options.OverrideAgentVersion,
			PolicyName:           options.PolicyName,
			DataStream:           options.DataStream,
			RunSetup:             options.RunSetup,
			RunTestsOnly:         options.RunTestsOnly,
			RunTearDown:          options.RunTearDown,
		}
		return NewKubernetesAgentDeployer(opts)
	}
	return nil, fmt.Errorf("unsupported agent deployer (name: %s)", agentDeployerName)
}

func selectAgentDeployerType(options FactoryOptions) (string, error) {
	devDeployPath, err := servicedeployer.FindDevDeployPath(servicedeployer.FactoryOptions{
		DataStreamRoot: options.DataStreamRoot,
		DevDeployDir:   options.DevDeployDir,
		PackageRoot:    options.PackageRoot,
	})
	if errors.Is(err, os.ErrNotExist) {
		return "default", nil
	}
	if err != nil {
		return "", fmt.Errorf("can't find \"%s\" directory: %w", options.DevDeployDir, err)
	}

	agentDeployerName, err := findAgentDeployer(devDeployPath, options.DeployerName)
	if errors.Is(err, os.ErrNotExist) || (err == nil && agentDeployerName == "") {
		logger.Debugf("Not agent deployer found, using default one")
		return "default", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to find agent deployer: %w", err)
	}
	// if package defines `_dev/deploy/docker` or `_dev/deploy/tf` folder to start their services,
	// it should be using the default agent deployer`
	if agentDeployerName == "docker" || agentDeployerName == "tf" {
		return "default", nil
	}

	return agentDeployerName, nil
}

func findAgentDeployer(devDeployPath, expectedDeployer string) (string, error) {
	names, err := servicedeployer.FindAllServiceDeployers(devDeployPath)
	if err != nil {
		return "", fmt.Errorf("failed to find service deployers in \"%s\": %w", devDeployPath, err)
	}
	deployers := slices.DeleteFunc(names, func(name string) bool {
		return expectedDeployer != "" && name != expectedDeployer
	})

	// If we have more than one agent deployer, we expect to find only one.
	if expectedDeployer != "" && len(deployers) != 1 {
		return "", fmt.Errorf("expected to find %q agent deployer in %q", expectedDeployer, devDeployPath)
	}

	// It is allowed to have no agent deployers
	if len(deployers) == 0 {
		return "", nil
	}

	if len(deployers) == 1 {
		return deployers[0], nil
	}

	return "", fmt.Errorf("expected to find only one agent deployer in \"%s\" (found %d agent deployers)", devDeployPath, len(deployers))
}
