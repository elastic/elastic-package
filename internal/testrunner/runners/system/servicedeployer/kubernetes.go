// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kind"
	"github.com/elastic/elastic-package/internal/kubectl"
	"github.com/elastic/elastic-package/internal/logger"
)

const elasticAgentManagedYamlURL = "https://raw.githubusercontent.com/elastic/beats/7.x/deploy/kubernetes/elastic-agent-managed-kubernetes.yaml"

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
	for _, file := range files {
		definitionPaths = append(definitionPaths, file)
	}
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

// downloadElasticAgentManagedYAML will download a url from a path and return the response body.
func downloadElasticAgentManagedYAML(url string) ([]byte, error) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get file from URL %s", url)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	logger.Debugf("status code when downloading elastic-agent-managed-kubernetes.yaml is %d", resp.StatusCode)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("downloading failed due to status code %d, resp body: %s", resp.StatusCode, string(b))
	}
	return b, nil
}

// getElasticAgentYAML retrieves elastic-agent-managed.yaml from upstream and modifies the file as needed
// to run locally.
func getElasticAgentYAML() ([]byte, error) {
	appConfig, err := install.Configuration()
	if err != nil {
		return nil, errors.Wrap(err, "can't read application configuration")
	}

	logger.Debugf("downloading elastic-agent-managed-kubernetes.yaml from %s", elasticAgentManagedYamlURL)
	// retry downloading elastic agent manifest for 5 times (sleep 10 seconds between each try) in case of error
	elasticAgentManagedYaml, err := retryDownloadElasticAgentManagedYAML(elasticAgentManagedYamlURL, 5, 10,
		downloadElasticAgentManagedYAML)
	if err != nil {
		return nil, errors.Wrapf(err, "downloading failed for file from source  %s", elasticAgentManagedYamlURL)
	}

	// Set regex to match fleet url from yaml file
	fleetURLRegex := regexp.MustCompile("http(s){0,1}:\\/\\/fleet-server:(\\d+)")
	// Replace fleet url
	elasticAgentManagedYaml = fleetURLRegex.ReplaceAll(elasticAgentManagedYaml, []byte("http://fleet-server:8220"))

	// Set regex to match image name from yaml file
	imageRegex := regexp.MustCompile("docker.elastic.co/beats/elastic-agent:\\d.+")
	// Replace image name
	elasticAgentManagedYaml = imageRegex.ReplaceAll(elasticAgentManagedYaml, []byte(appConfig.DefaultStackImageRefs().ElasticAgent))

	return elasticAgentManagedYaml, nil
}

// retryDownloadElasticAgentManagedYAML retries downloading elastic agent managed manifest for x attempts
// until there is no error and bytes of the file are more than 2000.
func retryDownloadElasticAgentManagedYAML(url string, attempts int, sleep time.Duration, f func(string) ([]byte, error)) (
	elasticAgentManagedYaml []byte, err error) {
	for i := 0; i < attempts; i++ {
		if i > 0 {
			logger.Debugf("retrying download attempt %d", i+1)
			time.Sleep(sleep * time.Second)
		}
		elasticAgentManagedYaml, err = f(url)
		if err == nil {
			logger.Debugf("downloaded %d bytes", len(elasticAgentManagedYaml))
			if len(elasticAgentManagedYaml) > 2000 {
				return elasticAgentManagedYaml, nil
			}
			err = fmt.Errorf("bytes downloaded should be more than 2000 but where: %d", len(elasticAgentManagedYaml))
			logger.Debugf("failed because %s", err)
		}
	}
	return nil,
		errors.Wrapf(err, "failed after %d unsuccessful attempts of downloading elastic-agent-managed-kubernetes.yaml", attempts)
}
