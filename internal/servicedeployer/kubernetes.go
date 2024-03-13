// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	_ "embed"
	"fmt"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/kind"
	"github.com/elastic/elastic-package/internal/kubectl"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

// KubernetesServiceDeployer is responsible for deploying resources in the Kubernetes cluster.
type KubernetesServiceDeployer struct {
	profile        *profile.Profile
	definitionsDir string
	stackVersion   string

	runSetup     bool
	runTestsOnly bool
	runTearDown  bool
}

type KubernetesServiceDeployerOptions struct {
	Profile        *profile.Profile
	DefinitionsDir string
	StackVersion   string

	RunSetup     bool
	RunTestsOnly bool
	RunTearDown  bool
}

type kubernetesDeployedService struct {
	ctxt ServiceContext

	definitionsDir string
}

func (s kubernetesDeployedService) TearDown() error {
	logger.Debugf("uninstall custom Kubernetes definitions (directory: %s)", s.definitionsDir)

	definitionPaths, err := findKubernetesDefinitions(s.definitionsDir)
	if err != nil {
		return fmt.Errorf("can't find Kubernetes definitions in given directory (path: %s): %w", s.definitionsDir, err)
	}

	if len(definitionPaths) == 0 {
		logger.Debugf("no custom definitions found (directory: %s). Nothing will be uninstalled.", s.definitionsDir)
		return nil
	}

	err = kubectl.Delete(definitionPaths)
	if err != nil {
		return fmt.Errorf("can't uninstall Kubernetes resources (path: %s): %w", s.definitionsDir, err)
	}
	return nil
}

func (s kubernetesDeployedService) Signal(_ string) error {
	return ErrNotSupported
}

func (s kubernetesDeployedService) ExitCode(_ string) (bool, int, error) {
	return false, -1, ErrNotSupported
}

func (s kubernetesDeployedService) Context() ServiceContext {
	return s.ctxt
}

func (s *kubernetesDeployedService) SetContext(sc ServiceContext) error {
	s.ctxt = sc
	return nil
}

var _ DeployedService = new(kubernetesDeployedService)

// NewKubernetesServiceDeployer function creates a new instance of KubernetesServiceDeployer.
func NewKubernetesServiceDeployer(opts KubernetesServiceDeployerOptions) (*KubernetesServiceDeployer, error) {
	return &KubernetesServiceDeployer{
		profile:        opts.Profile,
		definitionsDir: opts.DefinitionsDir,
		stackVersion:   opts.StackVersion,
		runSetup:       opts.RunSetup,
		runTestsOnly:   opts.RunTestsOnly,
		runTearDown:    opts.RunTearDown,
	}, nil
}

// SetUp function links the kind container with elastic-package-stack network, installs
// custom YAML definitions if any.
func (ksd KubernetesServiceDeployer) SetUp(ctxt ServiceContext) (DeployedService, error) {
	err := kind.VerifyContext()
	if err != nil {
		return nil, fmt.Errorf("kind context verification failed: %w", err)
	}

	if ksd.runTearDown || ksd.runTestsOnly {
		logger.Debug("Skip connect kind to Elastic stack network")
	} else {
		err = kind.ConnectToElasticStackNetwork(ksd.profile)
		if err != nil {
			return nil, fmt.Errorf("can't connect control plane to Elastic stack network: %w", err)
		}
	}

	if !ksd.runTearDown {
		err = ksd.installCustomDefinitions()
		if err != nil {
			return nil, fmt.Errorf("can't install custom definitions in the Kubernetes cluster: %w", err)
		}
	}

	ctxt.Name = kind.ControlPlaneContainerName
	ctxt.Hostname = kind.ControlPlaneContainerName
	// kind-control-plane is the name of the kind host where Pod is running since we use hostNetwork setting
	// to deploy Agent Pod. Because of this, hostname inside pod will be equal to the name of the k8s host.
	ctxt.Agent.Host.NamePrefix = "kind-control-plane"
	return &kubernetesDeployedService{
		ctxt:           ctxt,
		definitionsDir: ksd.definitionsDir,
	}, nil
}

func (ksd KubernetesServiceDeployer) installCustomDefinitions() error {
	logger.Debugf("install custom Kubernetes definitions (directory: %s)", ksd.definitionsDir)

	definitionPaths, err := findKubernetesDefinitions(ksd.definitionsDir)
	if err != nil {
		return fmt.Errorf("can't find Kubernetes definitions in given path: %s: %w", ksd.definitionsDir, err)
	}

	if len(definitionPaths) == 0 {
		logger.Debugf("no custom definitions found (path: %s). Nothing else will be installed.", ksd.definitionsDir)
		return nil
	}

	err = kubectl.Apply(definitionPaths)
	if err != nil {
		return fmt.Errorf("can't install custom definitions: %w", err)
	}
	return nil
}

var _ ServiceDeployer = new(KubernetesServiceDeployer)

func findKubernetesDefinitions(definitionsDir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(definitionsDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("can't read definitions directory (path: %s): %w", definitionsDir, err)
	}

	var definitionPaths []string
	definitionPaths = append(definitionPaths, files...)
	return definitionPaths, nil
}
