package servicedeployer

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	kindControlPlaneContainerName = "kind-control-plane"
)

// KubernetesServiceDeployer is responsible for deploying resources in the Kubernetes cluster.
type KubernetesServiceDeployer struct {
	definitionsDir string
}

// NewKubernetesServiceDeployer function creates a new instance of KubernetesServiceDeployer.
func NewKubernetesServiceDeployer(definitionsDir string) (*KubernetesServiceDeployer, error) {
	return &KubernetesServiceDeployer{
		definitionsDir: definitionsDir,
	}, nil
}

// SetUp function links the kind container with elastic-package-stack network, installs Elastic-Agent and optionally
// custom YAML definitions.
func (ksd KubernetesServiceDeployer) SetUp(ctxt ServiceContext) (DeployedService, error) {
	// TODO Check if "kubectl config current-context" == "kind-kind"

	// Find kind-control-plane container
	logger.Debugf("find \"%s\" container", kindControlPlaneContainerName)
	cmd := exec.Command("docker", "ps", "--filter", "name=kind-control-plane", "--format", "{{.ID}}")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	cids, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "could not find \"%s\" container to the stack network (stderr=%q)", kindControlPlaneContainerName, errOutput.String())
	}
	containerIDs := bytes.Split(bytes.TrimSpace(cids), []byte{'\n'})
	if len(containerIDs) != 1 {
		return nil, fmt.Errorf("expected single %s container, make sure you have run \"kind create cluster\" and the %s container is present", kindControlPlaneContainerName, kindControlPlaneContainerName)
	}
	kindControlPlaneContainerID := string(containerIDs[0])

	// Connect "kind" network with stack network (for the purpose of metrics collection)
	stackNetwork := fmt.Sprintf("%s_default", stack.DockerComposeProjectName)
	logger.Debugf("attaching service container %s (ID: %s) to stack network %s", kindControlPlaneContainerName, kindControlPlaneContainerID, stackNetwork)
	cmd = exec.Command("docker", "network", "connect", stackNetwork, kindControlPlaneContainerID)
	errOutput = new(bytes.Buffer)
	cmd.Stderr = errOutput
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "could not attach \"%s\" container to the stack network (stderr=%q)", kindControlPlaneContainerName, errOutput.String())
	}

	// Install Elastic-Agent in the cluster if it's not there
	logger.Debugf("attaching service container %s (ID: %s) to stack network %s", kindControlPlaneContainerName, kindControlPlaneContainerID, stackNetwork)
	elasticAgentFile, err := install.KubernetesDeployerElasticAgentFile()
	if err != nil {
		return nil, errors.Wrap(err, "can't locate docker compose file for Terraform deployer")
	}

	fmt.Println(elasticAgentFile) // TODO install

	// TODO install custom definitions
	// TODO Test execution: List cluster agent only
	// TODO uninstall custom definitions

	panic("implement me")
}

var _ ServiceDeployer = new(KubernetesServiceDeployer)
