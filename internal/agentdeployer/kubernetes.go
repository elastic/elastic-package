// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kind"
	"github.com/elastic/elastic-package/internal/kubectl"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

// KubernetesAgentDeployer is responsible for deploying resources in the Kubernetes cluster.
type KubernetesAgentDeployer struct {
	profile              *profile.Profile
	stackVersion         string
	overrideAgentVersion string
	policyName           string
	dataStream           string

	agentRunID string

	runSetup     bool
	runTestsOnly bool
	runTearDown  bool
}

type KubernetesAgentDeployerOptions struct {
	Profile              *profile.Profile
	StackVersion         string
	OverrideAgentVersion string
	PolicyName           string
	DataStream           string

	RunSetup     bool
	RunTestsOnly bool
	RunTearDown  bool
}

type kubernetesDeployedAgent struct {
	agentInfo    AgentInfo
	profile      *profile.Profile
	stackVersion string

	agentName            string
	overrideAgentVersion string
}

func (s kubernetesDeployedAgent) TearDown(ctx context.Context) error {
	elasticAgentManagedYaml, err := getElasticAgentYAML(ctx, s.profile, s.agentInfo, s.stackVersion, s.agentName, s.overrideAgentVersion)
	if err != nil {
		return fmt.Errorf("can't retrieve Kubernetes file for Elastic Agent: %w", err)
	}
	err = kubectl.DeleteStdin(ctx, elasticAgentManagedYaml)
	if err != nil {
		return fmt.Errorf("can't uninstall Kubernetes Elastic Agent resources: %w", err)
	}
	return nil
}

func (s kubernetesDeployedAgent) ExitCode(ctx context.Context) (bool, int, error) {
	return false, -1, ErrNotSupported
}

func (s kubernetesDeployedAgent) Info() AgentInfo {
	return s.agentInfo
}

func (s *kubernetesDeployedAgent) SetInfo(info AgentInfo) {
	s.agentInfo = info
}

// Logs returns the logs from the agent starting at the given time
func (s *kubernetesDeployedAgent) Logs(ctx context.Context, t time.Time) ([]byte, error) {
	return nil, nil
}

var _ DeployedAgent = new(kubernetesDeployedAgent)

// NewKubernetesAgentDeployer function creates a new instance of KubernetesAgentDeployer.
func NewKubernetesAgentDeployer(opts KubernetesAgentDeployerOptions) (*KubernetesAgentDeployer, error) {
	return &KubernetesAgentDeployer{
		profile:              opts.Profile,
		stackVersion:         opts.StackVersion,
		overrideAgentVersion: opts.OverrideAgentVersion,
		policyName:           opts.PolicyName,
		dataStream:           opts.DataStream,
		runSetup:             opts.RunSetup,
		runTestsOnly:         opts.RunTestsOnly,
		runTearDown:          opts.RunTearDown,
	}, nil
}

// SetUp function links the kind container with elastic-package-stack network, installs Elastic-Agent and optionally
// custom YAML definitions.
func (ksd *KubernetesAgentDeployer) SetUp(ctx context.Context, agentInfo AgentInfo) (DeployedAgent, error) {
	ksd.agentRunID = agentInfo.Test.RunID

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

	agentName := ksd.agentName()
	if ksd.runTearDown || ksd.runTestsOnly {
		logger.Debug("Skip install Elastic Agent in cluster")
	} else {
		err = installElasticAgentInCluster(ctx, ksd.profile, agentInfo, ksd.stackVersion, agentName, ksd.overrideAgentVersion)
		if err != nil {
			return nil, fmt.Errorf("can't install Elastic-Agent in the Kubernetes cluster: %w", err)
		}
	}

	agentInfo.Name = kind.ControlPlaneContainerName
	agentInfo.Hostname = kind.ControlPlaneContainerName
	// kind-control-plane is the name of the kind host where Pod is running since we use hostNetwork setting
	// to deploy Agent Pod. Because of this, hostname inside pod will be equal to the name of the k8s host.
	agentInfo.Agent.Host.NamePrefix = "kind-control-plane"
	return &kubernetesDeployedAgent{
		agentInfo:            agentInfo,
		profile:              ksd.profile,
		stackVersion:         ksd.stackVersion,
		agentName:            agentName,
		overrideAgentVersion: ksd.overrideAgentVersion,
	}, nil
}

func (ksd *KubernetesAgentDeployer) agentName() string {
	name := "elastic-agent"
	if ksd.dataStream != "" && ksd.dataStream != "." {
		name = fmt.Sprintf("%s-%s", name, strings.ReplaceAll(ksd.dataStream, "_", "-"))
	}
	if ksd.agentRunID != "" {
		name = fmt.Sprintf("%s-%s", name, ksd.agentRunID)
	}
	return name
}

var _ AgentDeployer = new(KubernetesAgentDeployer)

func installElasticAgentInCluster(ctx context.Context, profile *profile.Profile, agentInfo AgentInfo, stackVersion, agentName, agentVersion string) error {
	logger.Debug("install Elastic Agent in the Kubernetes cluster")

	elasticAgentManagedYaml, err := getElasticAgentYAML(ctx, profile, agentInfo, stackVersion, agentName, agentVersion)
	if err != nil {
		return fmt.Errorf("can't retrieve Kubernetes file for Elastic Agent: %w", err)
	}

	err = kubectl.ApplyStdin(ctx, elasticAgentManagedYaml)
	if err != nil {
		return fmt.Errorf("can't install Elastic-Agent in Kubernetes cluster: %w", err)
	}

	// DEBUG DaemonSet is not ready: kube-system/elastic-agent. 0 out of 1 expected pods have been scheduled

	return nil
}

//go:embed _static/elastic-agent-managed.yaml.tmpl
var elasticAgentManagedYamlTmpl string

func getElasticAgentYAML(ctx context.Context, profile *profile.Profile, agentInfo AgentInfo, stackVersion, agentName, overrideAgentVersion string) ([]byte, error) {
	logger.Debugf("Prepare YAML definition for Elastic Agent running in stack v%s", stackVersion)
	config, err := stack.LoadConfig(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from profile: %w", err)
	}
	fleetURL := "https://fleet-server:8220"
	kibanaURL := "https://kibana:5601"
	if config.Provider != stack.ProviderCompose {
		kibanaURL = config.KibanaHost
	}
	if url, ok := config.Parameters[stack.ParamServerlessFleetURL]; ok {
		fleetURL = url
	}
	if version, ok := config.Parameters[stack.ParamServerlessLocalStackVersion]; ok {
		stackVersion = version
	}

	enrollmentToken := ""
	if config.ElasticsearchAPIKey != "" {
		// TODO: Review if this is the correct place to get the enrollment token.
		kibanaClient, err := stack.NewKibanaClientFromProfile(profile)
		if err != nil {
			return nil, fmt.Errorf("failed to create kibana client: %w", err)
		}
		enrollmentToken, err = kibanaClient.GetEnrollmentTokenForPolicyID(ctx, agentInfo.Policy.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get enrollment token for policy %q: %w", agentInfo.Policy.Name, err)
		}
	}

	// default to stack version if no override is provided
	agentVersion := stackVersion
	if overrideAgentVersion != "" {
		agentVersion = overrideAgentVersion
	}

	appConfig, err := install.Configuration(install.OptionWithAgentVersion(agentVersion))
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
		"fleetURL":                    fleetURL,
		"kibanaURL":                   kibanaURL,
		"username":                    config.ElasticsearchUsername,
		"password":                    config.ElasticsearchPassword,
		"enrollmentToken":             enrollmentToken,
		"caCertPem":                   caCert,
		"elasticAgentImage":           appConfig.StackImageRefs().ElasticAgent,
		"elasticAgentTokenPolicyName": getTokenPolicyName(agentVersion, agentInfo.Policy.Name),
		"agentName":                   agentName,
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

// getTokenPolicyName function returns the policy name for the >= 8.x Elastic stacks. The agent's policy
// is predefined in the Kibana configuration file. The logic is not present in older stacks and it uses
// the default policy in Kibana (empty string).
func getTokenPolicyName(agentVersion, policyName string) string {
	if strings.HasPrefix(agentVersion, "7.") {
		return ""
	}
	return policyName
}
