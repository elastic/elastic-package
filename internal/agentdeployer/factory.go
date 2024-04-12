// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"errors"
	"fmt"
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
	devDeployPath, err := FindDevDeployPath(options)
	if err != nil {
		return nil, fmt.Errorf("can't find \"%s\" directory: %w", options.DevDeployDir, err)
	}

	agentDeployerName, err := findAgentDeployer(devDeployPath)
	if err != nil {
		return nil, fmt.Errorf("can't find any valid agent deployer: %w", err)
	}

	switch agentDeployerName {
	// if package defines `docker`  folder to start their services, it should be
	// using the default agent deployer`
	case "docker", "tf", "none":
		if options.Type != TypeTest {
			return nil, fmt.Errorf("agent deployer is not supported for type %s", options.Type)
		}
		opts := DockerComposeAgentDeployerOptions{
			Profile:           options.Profile,
			DockerComposeFile: "", // TODO: Allow other docker-compose files to apply overrides
			StackVersion:      options.StackVersion,
			PackageName:       options.PackageName,
			PolicyName:        options.PolicyName,
			DataStream:        options.DataStream,
			RunTearDown:       options.RunTearDown,
			RunTestsOnly:      options.RunTestsOnly,
		}
		return NewCustomAgentDeployer(opts)
	case "agent":
		// FIXME: should this be just carried out by service deployer?
		// FIXME: this docker-compose scenario contains both agent and service
		return nil, nil
	case "k8s":
		agentDeployerPath := filepath.Join(devDeployPath, agentDeployerName)
		if _, err := os.Stat(agentDeployerPath); err == nil {
			opts := KubernetesAgentDeployerOptions{
				Profile:        options.Profile,
				DefinitionsDir: agentDeployerPath,
				StackVersion:   options.StackVersion,
				PolicyName:     options.PolicyName,
				DataStream:     options.DataStream,
				RunSetup:       options.RunSetup,
				RunTestsOnly:   options.RunTestsOnly,
				RunTearDown:    options.RunTearDown,
			}
			return NewKubernetesAgentDeployer(opts)
		}
	}
	return nil, fmt.Errorf("unsupported agent deployer (name: %s)", agentDeployerName)
}

// FindDevDeployPath function returns a path reference to the "_dev/deploy" directory.
func FindDevDeployPath(options FactoryOptions) (string, error) {
	dataStreamDevDeployPath := filepath.Join(options.DataStreamRootPath, options.DevDeployDir)
	if _, err := os.Stat(dataStreamDevDeployPath); err == nil {
		return dataStreamDevDeployPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat failed for data stream (path: %s): %w", dataStreamDevDeployPath, err)
	}

	packageDevDeployPath := filepath.Join(options.PackageRootPath, options.DevDeployDir)
	if _, err := os.Stat(packageDevDeployPath); err == nil {
		return packageDevDeployPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat failed for package (path: %s): %w", packageDevDeployPath, err)
	}

	return "", fmt.Errorf("\"%s\" directory doesn't exist", options.DevDeployDir)
}

func findAgentDeployer(devDeployPath string) (string, error) {
	fis, err := os.ReadDir(devDeployPath)
	if err != nil {
		return "", fmt.Errorf("can't read directory (path: %s): %w", devDeployPath, err)
	}

	var folders []os.DirEntry
	for _, fi := range fis {
		if fi.IsDir() {
			folders = append(folders, fi)
		}
	}

	if len(folders) != 1 {
		return "", fmt.Errorf("expected to find only one agent deployer in \"%s\"", devDeployPath)
	}
	return folders[0].Name(), nil
}
