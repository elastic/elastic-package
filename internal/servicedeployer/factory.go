// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/elastic/elastic-package/internal/profile"
)

const (
	TypeTest  = "test"
	TypeBench = "bench"
)

// FactoryOptions defines options used to create an instance of a service deployer.
type FactoryOptions struct {
	WorkDir string
	Profile *profile.Profile

	PackageRootPath        string
	DataStreamRootPath     string
	DevDeployDir           string
	Type                   string
	StackVersion           string
	DeployIndependentAgent bool

	PolicyName string

	DeployerName string

	Variant string

	RunTearDown  bool
	RunTestsOnly bool
	RunSetup     bool
}

// Factory chooses the appropriate service runner for the given data stream, depending
// on service configuration files defined in the package or data stream.
func Factory(options FactoryOptions) (ServiceDeployer, error) {
	devDeployPath, err := FindDevDeployPath(options)
	if err != nil {
		return nil, fmt.Errorf("can't find \"%s\" directory: %w", options.DevDeployDir, err)
	}

	serviceDeployerName, err := findServiceDeployer(devDeployPath, options.DeployerName)
	if err != nil {
		return nil, fmt.Errorf("can't find any valid service deployer: %w", err)
	}
	// It's allowed to not define a service deployer in system tests
	// if deployerName is not defined in the test configuration.
	if serviceDeployerName == "" {
		return nil, nil
	}

	serviceDeployerPath := filepath.Join(devDeployPath, serviceDeployerName)

	switch serviceDeployerName {
	case "k8s":
		if _, err := os.Stat(serviceDeployerPath); err == nil {
			opts := KubernetesServiceDeployerOptions{
				Profile:                options.Profile,
				DefinitionsDir:         serviceDeployerPath,
				StackVersion:           options.StackVersion,
				PolicyName:             options.PolicyName,
				RunSetup:               options.RunSetup,
				RunTestsOnly:           options.RunTestsOnly,
				RunTearDown:            options.RunTearDown,
				DeployIndependentAgent: options.DeployIndependentAgent,
			}
			return NewKubernetesServiceDeployer(opts)
		}
	case "docker":
		dockerComposeYMLPath := filepath.Join(serviceDeployerPath, "docker-compose.yml")
		if _, err := os.Stat(dockerComposeYMLPath); err == nil {
			sv, err := useServiceVariant(devDeployPath, options.Variant)
			if err != nil {
				return nil, fmt.Errorf("can't use service variant: %w", err)
			}
			opts := DockerComposeServiceDeployerOptions{
				WorkDir:                options.WorkDir,
				Profile:                options.Profile,
				YmlPaths:               []string{dockerComposeYMLPath},
				Variant:                sv,
				RunTearDown:            options.RunTearDown,
				RunTestsOnly:           options.RunTestsOnly,
				DeployIndependentAgent: options.DeployIndependentAgent,
			}
			return NewDockerComposeServiceDeployer(opts)
		}
	case "agent":
		// FIXME: This docker-compose scenario contains also the definition of the elastic-agent container
		if options.Type != TypeTest {
			return nil, fmt.Errorf("agent deployer is not supported for type %s", options.Type)
		}
		customAgentCfgYMLPath := filepath.Join(serviceDeployerPath, "custom-agent.yml")
		if _, err := os.Stat(customAgentCfgYMLPath); err != nil {
			return nil, fmt.Errorf("can't find expected file custom-agent.yml: %w", err)
		}
		policyName := getTokenPolicyName(options.StackVersion, options.PolicyName)

		opts := CustomAgentDeployerOptions{
			WorkDir:           options.WorkDir,
			Profile:           options.Profile,
			DockerComposeFile: customAgentCfgYMLPath,
			StackVersion:      options.StackVersion,
			PolicyName:        policyName,

			RunTearDown:  options.RunTearDown,
			RunTestsOnly: options.RunTestsOnly,
		}
		return NewCustomAgentDeployer(opts)
	case "tf":
		if options.RunSetup || options.RunTearDown || options.RunTestsOnly {
			return nil, errors.New("terraform service deployer not supported to run by steps")
		}
		if _, err := os.Stat(serviceDeployerPath); err == nil {
			opts := TerraformServiceDeployerOptions{
				WorkDir:        options.WorkDir,
				DefinitionsDir: serviceDeployerPath,
			}
			return NewTerraformServiceDeployer(opts)
		}
	}
	return nil, fmt.Errorf("unsupported service deployer (name: %s)", serviceDeployerName)
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

func FindAllServiceDeployers(devDeployPath string) ([]string, error) {
	fis, err := os.ReadDir(devDeployPath)
	if err != nil {
		return nil, fmt.Errorf("can't read directory (path: %s): %w", devDeployPath, err)
	}

	var names []string
	for _, fi := range fis {
		if fi.IsDir() {
			names = append(names, fi.Name())
		}
	}

	return names, nil
}

func findServiceDeployer(devDeployPath, expectedDeployer string) (string, error) {
	names, err := FindAllServiceDeployers(devDeployPath)
	if err != nil {
		return "", fmt.Errorf("failed to find service deployers in %q: %w", devDeployPath, err)
	}
	deployers := slices.DeleteFunc(names, func(name string) bool {
		return expectedDeployer != "" && name != expectedDeployer
	})

	if len(deployers) == 1 {
		return deployers[0], nil
	}

	if expectedDeployer != "" {
		return "", fmt.Errorf("expected to find %q service deployer in %q", expectedDeployer, devDeployPath)
	}

	// If "_dev/deploy" directory exists, but it is empty. It does not have any service deployer,
	// package-spec does not disallow to be empty this folder.
	if len(deployers) == 0 {
		return "", nil
	}

	return "", fmt.Errorf("expected to find only one service deployer in %q (found %d service deployers)", devDeployPath, len(deployers))
}
