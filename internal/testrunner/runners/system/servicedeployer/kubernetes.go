// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"bytes"
	_ "embed"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kind"
	"github.com/elastic/elastic-package/internal/kubectl"
	"github.com/elastic/elastic-package/internal/logger"
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

	err = kubectl.Delete(definitionPaths...)
	if err != nil {
		return errors.Wrapf(err, "can't uninstall Kubernetes resources (path: %s)", s.definitionsDir)
	}
	return nil
}

func (s kubernetesDeployedService) Signal(_ string) error {
	return errors.New("signal is not supported")
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
func NewKubernetesServiceDeployer(definitionsDir string) (*KubernetesServiceDeployer, error) {
	return &KubernetesServiceDeployer{
		definitionsDir: definitionsDir,
	}, nil
}

// SetUp function links the kind container with elastic-package-stack network, installs Elastic-Agent and optionally
// custom YAML definitions.
func (ksd KubernetesServiceDeployer) SetUp(ctxt ServiceContext) (DeployedService, error) {
	err := kind.VerifyContext()
	if err != nil {
		return nil, errors.Wrap(err, "kind context verification failed")
	}

	err = kind.ConnectToElasticStackNetwork()
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
		return errors.Wrapf(err, "can't find Kubernetes definitions in given directory (path: %s)", ksd.definitionsDir)
	}

	if len(definitionPaths) == 0 {
		logger.Debugf("no custom definitions found (directory: %s). Nothing else will be installed.", ksd.definitionsDir)
		return nil
	}

	err = kubectl.Apply(definitionPaths...)
	if err != nil {
		return errors.Wrap(err, "can't install custom definitions")
	}
	return nil
}

var _ ServiceDeployer = new(KubernetesServiceDeployer)

func findKubernetesDefinitions(definitionsDir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(definitionsDir, "*.yaml"))
	if err != nil {
		return nil, errors.Wrapf(err, "can't read definitions directory (path: %s)", definitionsDir)
	}

	var definitionPaths []string
	definitionPaths = append(definitionPaths, files...)
	return definitionPaths, nil
}

func installElasticAgentInCluster() error {
	logger.Debug("install Elastic Agent in the Kubernetes cluster")

	elasticAgentManagedYaml, err := getElasticAgentYAML()
	if err != nil {
		return errors.Wrap(err, "can't retrieve Kubernetes file for Elastic Agent")
	}

	err = kubectl.ApplyStdin(elasticAgentManagedYaml)
	if err != nil {
		return errors.Wrap(err, "can't install Elastic-Agent in Kubernetes cluster")
	}
	return nil
}

//go:embed elastic-agent-managed.yaml.tmpl
var elasticAgentManagedYamlTmpl string

func getElasticAgentYAML() ([]byte, error) {
	appConfig, err := install.Configuration()
	if err != nil {
		return nil, errors.Wrap(err, "can't read application configuration")
	}

	tmpl := template.Must(template.New("elastic-agent.yml").Parse(elasticAgentManagedYamlTmpl))

	var elasticAgentYaml bytes.Buffer
	err = tmpl.Execute(&elasticAgentYaml, map[string]string{
		"fleetURL":          "http://fleet-server:8220",
		"elasticAgentImage": appConfig.DefaultStackImageRefs().ElasticAgent,
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't generate elastic agent manifest")
	}

	return elasticAgentYaml.Bytes(), nil
}
