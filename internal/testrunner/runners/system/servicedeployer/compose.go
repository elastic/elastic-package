package servicedeployer

import (
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/logger"
)

// DockerComposeRunner knows how to setup and teardown a service defined via
// a docker-compose.yml file.
type DockerComposeRunner struct {
	ymlPath string
	network string
}

// NewDockerComposeRunner returns a new instance of a DockerComposeRunner.
func NewDockerComposeRunner(ymlPath string) (*DockerComposeRunner, error) {
	return &DockerComposeRunner{
		ymlPath: ymlPath,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (r *DockerComposeRunner) SetUp(ctxt common.MapStr) (common.MapStr, error) {
	logger.Infof("Setting up service using docker compose runner")
	//v, err := ctxt.GetValue("docker.compose.network")
	//if err != nil {
	//	return ctxt, errors.Wrap(err, "could not determine docker compose network to join")
	//}
	//
	//network, ok := v.(string)
	//if !ok {
	//	return ctxt, fmt.Errorf("expected docker compose network name to be a string, got: %v", v)
	//}
	//r.network = network

	// TODO
	return ctxt, nil
}

// TearDown tears down the service.
func (r *DockerComposeRunner) TearDown(ctxt common.MapStr) error {
	logger.Infof("Tearing down service using docker compose runner")
	// TODO
	return nil
}
