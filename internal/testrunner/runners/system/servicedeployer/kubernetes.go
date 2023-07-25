// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/kind"
	"github.com/elastic/elastic-package/internal/kubectl"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

// KubernetesServiceDeployer is responsible for deploying resources in the Kubernetes cluster.
type KubernetesServiceDeployer struct {
	profile        *profile.Profile
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
func NewKubernetesServiceDeployer(profile *profile.Profile, definitionsPath string) (*KubernetesServiceDeployer, error) {
	return &KubernetesServiceDeployer{
		profile:        profile,
		definitionsDir: definitionsPath,
	}, nil
}

// SetUp function links the kind container with elastic-package-stack network, installs Elastic-Agent and optionally
// custom YAML definitions.
func (ksd KubernetesServiceDeployer) SetUp(ctxt ServiceContext) (DeployedService, error) {
	err := kind.VerifyContext()
	if err != nil {
		return nil, fmt.Errorf("kind context verification failed: %w", err)
	}

	err = kind.ConnectToElasticStackNetwork(ksd.profile)
	if err != nil {
		return nil, fmt.Errorf("can't connect control plane to Elastic stack network: %w", err)
	}

	err = installElasticAgentInCluster()
	if err != nil {
		return nil, fmt.Errorf("can't install Elastic-Agent in the Kubernetes cluster: %w", err)
	}

	err = ksd.installCustomDefinitions()
	if err != nil {
		return nil, fmt.Errorf("can't install custom definitions in the Kubernetes cluster: %w", err)
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

func installElasticAgentInCluster() error {
	logger.Debug("install Elastic Agent in the Kubernetes cluster")

	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return fmt.Errorf("can't create Kibana client: %w", err)
	}

	stackVersion, err := kibanaClient.Version()
	if err != nil {
		return fmt.Errorf("can't read Kibana injected metadata: %w", err)
	}

	elasticAgentManagedYaml, err := getElasticAgentYAML(stackVersion.Version())
	if err != nil {
		return fmt.Errorf("can't retrieve Kubernetes file for Elastic Agent: %w", err)
	}

	err = kubectl.ApplyStdin(elasticAgentManagedYaml)
	if err != nil {
		return fmt.Errorf("can't install Elastic-Agent in Kubernetes cluster: %w", err)
	}
	return nil
}

//go:embed elastic-agent-managed.yaml.tmpl
var elasticAgentManagedYamlTmpl string

func getElasticAgentYAML(stackVersion string) ([]byte, error) {
	logger.Debugf("Prepare YAML definition for Elastic Agent running in stack v%s", stackVersion)

	appConfig, err := install.Configuration()
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	caCert, err := readCACertBase64()
	if err != nil {
		return nil, fmt.Errorf("can't read certificate authority file: %w", err)
	}

	tmpl := template.Must(template.New("elastic-agent.yml").Parse(elasticAgentManagedYamlTmpl))

	var elasticAgentYaml bytes.Buffer
	err = tmpl.Execute(&elasticAgentYaml, map[string]string{
		"fleetURL":                    "https://fleet-server:8220",
		"kibanaURL":                   "https://kibana:5601",
		"caCertPem":                   caCert,
		"elasticAgentImage":           appConfig.StackImageRefs(stackVersion).ElasticAgent,
		"elasticAgentTokenPolicyName": getTokenPolicyName(stackVersion),
	})
	if err != nil {
		return nil, fmt.Errorf("can't generate elastic agent manifest: %w", err)
	}

	return elasticAgentYaml.Bytes(), nil
}

func readCACertBase64() (string, error) {
	caCertPath, ok := os.LookupEnv(stack.CACertificateEnv)
	if !ok {
		return "", fmt.Errorf("%s not defined", stack.CACertificateEnv)
	}

	d, err := os.ReadFile(caCertPath)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(d), nil
}

// getTokenPolicyName function returns the policy name for the 8.x Elastic stack. The agent's policy
// is predefined in the Kibana configuration file. The logic is not present in older stacks.
func getTokenPolicyName(stackVersion string) string {
	if strings.HasPrefix(stackVersion, "8.") {
		return "Elastic-Agent (elastic-package)"
	}
	return ""
}
