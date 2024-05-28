// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/profile"
)

const (
	TypeTest  = "test"
	TypeBench = "bench"
)

// FactoryOptions defines options used to create an instance of a service deployer.
type FactoryOptions struct {
	Logger *slog.Logger

	Profile *profile.Profile

	PackageRootPath    string
	DataStreamRootPath string
	DevDeployDir       string
	Type               string
	StackVersion       string
	PolicyName         string

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
			Profile:      options.Profile,
			StackVersion: options.StackVersion,
			PackageName:  options.PackageName,
			PolicyName:   options.PolicyName,
			DataStream:   options.DataStream,
			RunTearDown:  options.RunTearDown,
			RunTestsOnly: options.RunTestsOnly,
			Logger:       options.Logger,
		}
		return NewCustomAgentDeployer(opts)
	case "agent":
		// FIXME: should this be just carried out by service deployer?
		// FIXME: this docker-compose scenario contains both agent and service
		return nil, nil
	case "k8s":
		opts := KubernetesAgentDeployerOptions{
			Profile:      options.Profile,
			StackVersion: options.StackVersion,
			PolicyName:   options.PolicyName,
			DataStream:   options.DataStream,
			RunSetup:     options.RunSetup,
			RunTestsOnly: options.RunTestsOnly,
			RunTearDown:  options.RunTearDown,
			Logger:       options.Logger,
		}
		return NewKubernetesAgentDeployer(opts)
	}
	return nil, fmt.Errorf("unsupported agent deployer (name: %s)", agentDeployerName)
}

func selectAgentDeployerType(options FactoryOptions) (string, error) {
	devDeployPath, err := FindDevDeployPath(options)
	if errors.Is(err, os.ErrNotExist) {
		return "default", nil
	}
	if err != nil {
		return "", fmt.Errorf("can't find \"%s\" directory: %w", options.DevDeployDir, err)
	}

	agentDeployerNames, err := findAgentDeployers(devDeployPath)
	if errors.Is(err, os.ErrNotExist) || len(agentDeployerNames) == 0 {
		options.Logger.Debug("Not agent deployer found, using default one")
		return "default", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to find agent deployer: %w", err)
	}
	if len(agentDeployerNames) != 1 {
		return "", fmt.Errorf("expected to find only one agent deployer in \"%s\"", devDeployPath)
	}
	agentDeployerName := agentDeployerNames[0]

	// if package defines `_dev/deploy/docker` or `_dev/deploy/tf` folder to start their services,
	// it should be using the default agent deployer`
	if agentDeployerName == "docker" || agentDeployerName == "tf" {
		return "default", nil
	}

	return agentDeployerName, nil
}

// FindDevDeployPath function returns a path reference to the "_dev/deploy" directory.
func FindDevDeployPath(options FactoryOptions) (string, error) {
	dataStreamDevDeployPath := filepath.Join(options.DataStreamRootPath, options.DevDeployDir)
	info, err := os.Stat(dataStreamDevDeployPath)
	if err == nil && info.IsDir() {
		return dataStreamDevDeployPath, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat failed for data stream (path: %s): %w", dataStreamDevDeployPath, err)
	}

	packageDevDeployPath := filepath.Join(options.PackageRootPath, options.DevDeployDir)
	info, err = os.Stat(packageDevDeployPath)
	if err == nil && info.IsDir() {
		return packageDevDeployPath, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat failed for package (path: %s): %w", packageDevDeployPath, err)
	}

	return "", fmt.Errorf("\"%s\" %w", options.DevDeployDir, os.ErrNotExist)
}

func findAgentDeployers(devDeployPath string) ([]string, error) {
	fis, err := os.ReadDir(devDeployPath)
	if err != nil {
		return nil, fmt.Errorf("can't read directory (path: %s): %w", devDeployPath, err)
	}

	var folders []os.DirEntry
	for _, fi := range fis {
		if fi.IsDir() {
			folders = append(folders, fi)
		}
	}

	var names []string
	for _, folder := range folders {
		names = append(names, folder.Name())
	}
	return names, nil
}
