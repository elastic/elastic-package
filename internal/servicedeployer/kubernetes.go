// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/elastic/elastic-package/internal/install"
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
	stackVersion   string
	policyName     string

	deployIndependentAgent bool

	runSetup     bool
	runTestsOnly bool
	runTearDown  bool
}

type KubernetesServiceDeployerOptions struct {
	Profile        *profile.Profile
	DefinitionsDir string
	StackVersion   string
	PolicyName     string

	DeployIndependentAgent bool

	RunSetup     bool
	RunTestsOnly bool
	RunTearDown  bool
}

type kubernetesDeployedService struct {
	svcInfo      ServiceInfo
	stackVersion string
	profile      *profile.Profile
	policyName   string

	deployIndependentAgent bool

	definitionsDir string
}

func (s kubernetesDeployedService) TearDown(ctx context.Context) error {
	if !s.deployIndependentAgent {
		logger.Debug("Uninstall Elastic Agent Kubernetes")
		elasticAgentManagedYaml, err := getElasticAgentYAML(s.profile, s.stackVersion, s.policyName)
		if err != nil {
			return fmt.Errorf("can't retrieve Kubernetes file for Elastic Agent: %w", err)
		}
		err = kubectl.DeleteStdin(ctx, elasticAgentManagedYaml)
		if err != nil {
			return fmt.Errorf("can't uninstall Elastic Agent Kubernetes resources (path: %s): %w", s.definitionsDir, err)
		}
	}

	logger.Debugf("Uninstall custom Kubernetes definitions (directory: %s)", s.definitionsDir)
	definitionPaths, err := findKubernetesDefinitions(s.definitionsDir)
	if err != nil {
		return fmt.Errorf("can't find Kubernetes definitions in given directory (path: %s): %w", s.definitionsDir, err)
	}

	if len(definitionPaths) == 0 {
		logger.Debugf("no custom definitions found (directory: %s). Nothing will be uninstalled.", s.definitionsDir)
		return nil
	}

	err = kubectl.Delete(ctx, definitionPaths)
	if err != nil {
		return fmt.Errorf("can't uninstall Kubernetes resources (path: %s): %w", s.definitionsDir, err)
	}

	return nil
}

func (s kubernetesDeployedService) Signal(_ context.Context, _ string) error {
	return ErrNotSupported
}

func (s kubernetesDeployedService) ExitCode(_ context.Context, _ string) (bool, int, error) {
	return false, -1, ErrNotSupported
}

func (s kubernetesDeployedService) Info() ServiceInfo {
	return s.svcInfo
}

func (s *kubernetesDeployedService) SetInfo(sc ServiceInfo) error {
	s.svcInfo = sc
	return nil
}

var _ DeployedService = new(kubernetesDeployedService)

// NewKubernetesServiceDeployer function creates a new instance of KubernetesServiceDeployer.
func NewKubernetesServiceDeployer(opts KubernetesServiceDeployerOptions) (*KubernetesServiceDeployer, error) {
	return &KubernetesServiceDeployer{
		profile:                opts.Profile,
		definitionsDir:         opts.DefinitionsDir,
		stackVersion:           opts.StackVersion,
		policyName:             opts.PolicyName,
		runSetup:               opts.RunSetup,
		runTestsOnly:           opts.RunTestsOnly,
		runTearDown:            opts.RunTearDown,
		deployIndependentAgent: opts.DeployIndependentAgent,
	}, nil
}

// SetUp function links the kind container with elastic-package-stack network, installs Elastic-Agent and optionally
// custom YAML definitions.
func (ksd KubernetesServiceDeployer) SetUp(ctx context.Context, svcInfo ServiceInfo) (DeployedService, error) {
	err := kind.VerifyContext(ctx)
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

	if ksd.runTearDown || ksd.runTestsOnly || ksd.deployIndependentAgent {
		logger.Debug("Skip install Elastic Agent in cluster")
	} else {
		err = installElasticAgentInCluster(ctx, ksd.profile, ksd.stackVersion, ksd.policyName)
		if err != nil {
			return nil, fmt.Errorf("can't install Elastic-Agent in the Kubernetes cluster: %w", err)
		}
	}

	if !ksd.runTearDown {
		err = ksd.installCustomDefinitions(ctx)
		if err != nil {
			return nil, fmt.Errorf("can't install custom definitions in the Kubernetes cluster: %w", err)
		}
	}

	svcInfo.Agent.Independent = true
	svcInfo.Name = kind.ControlPlaneContainerName
	svcInfo.Hostname = kind.ControlPlaneContainerName
	// kind-control-plane is the name of the kind host where Pod is running since we use hostNetwork setting
	// to deploy Agent Pod. Because of this, hostname inside pod will be equal to the name of the k8s host.
	svcInfo.Agent.Host.NamePrefix = "kind-control-plane"
	return &kubernetesDeployedService{
		svcInfo:                svcInfo,
		definitionsDir:         ksd.definitionsDir,
		stackVersion:           ksd.stackVersion,
		profile:                ksd.profile,
		deployIndependentAgent: ksd.deployIndependentAgent,
		policyName:             ksd.policyName,
	}, nil
}

func (ksd KubernetesServiceDeployer) installCustomDefinitions(ctx context.Context) error {
	logger.Debugf("install custom Kubernetes definitions (directory: %s)", ksd.definitionsDir)

	definitionPaths, err := findKubernetesDefinitions(ksd.definitionsDir)
	if err != nil {
		return fmt.Errorf("can't find Kubernetes definitions in given path: %s: %w", ksd.definitionsDir, err)
	}

	if len(definitionPaths) == 0 {
		logger.Debugf("no custom definitions found (path: %s). Nothing else will be installed.", ksd.definitionsDir)
		return nil
	}

	err = kubectl.Apply(ctx, definitionPaths)
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

func installElasticAgentInCluster(ctx context.Context, profile *profile.Profile, stackVersion, policyName string) error {
	logger.Debug("install Elastic Agent in the Kubernetes cluster")

	elasticAgentManagedYaml, err := getElasticAgentYAML(profile, stackVersion, policyName)
	if err != nil {
		return fmt.Errorf("can't retrieve Kubernetes file for Elastic Agent: %w", err)
	}

	err = kubectl.ApplyStdin(ctx, elasticAgentManagedYaml)
	if err != nil {
		return fmt.Errorf("can't install Elastic-Agent in Kubernetes cluster: %w", err)
	}
	return nil
}

//go:embed _static/elastic-agent-managed.yaml.tmpl
var elasticAgentManagedYamlTmpl string

func getElasticAgentYAML(profile *profile.Profile, stackVersion, policyName string) ([]byte, error) {
	logger.Debugf("Prepare YAML definition for Elastic Agent running in stack v%s", stackVersion)

	appConfig, err := install.Configuration(install.OptionWithStackVersion(stackVersion))
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	caCert, err := readCACertBase64(profile)
	if err != nil {
		return nil, fmt.Errorf("can't read certificate authority file: %w", err)
	}

	tmpl := template.Must(template.New("elastic-agent.yml").Parse(elasticAgentManagedYamlTmpl))

	var elasticAgentYaml bytes.Buffer
	err = tmpl.Execute(&elasticAgentYaml, map[string]string{
		"fleetURL":                    "https://fleet-server:8220",
		"kibanaURL":                   "https://kibana:5601",
		"caCertPem":                   caCert,
		"elasticAgentImage":           appConfig.StackImageRefs().ElasticAgent,
		"elasticAgentTokenPolicyName": getTokenPolicyName(stackVersion, policyName),
	})
	if err != nil {
		return nil, fmt.Errorf("can't generate elastic agent manifest: %w", err)
	}

	return elasticAgentYaml.Bytes(), nil
}

func readCACertBase64(profile *profile.Profile) (string, error) {
	caCertPath, err := stack.FindCACertificate(profile)
	if err != nil {
		return "", fmt.Errorf("can't locate CA certificate: %w", err)
	}

	d, err := os.ReadFile(caCertPath)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(d), nil
}

// getTokenPolicyName function returns the policy name for the 8.x or later Elastic stacks. The agent's policy
// is predefined in the Kibana configuration file. The logic is not present in older stacks.
func getTokenPolicyName(stackVersion, policyName string) string {
	if strings.HasPrefix(stackVersion, "7.") {
		return ""
	}
	if policyName == "" {
		policyName = defaulFleetTokenPolicyName
	}
	// For 8.x and later, we return the given policy name
	return policyName
}
