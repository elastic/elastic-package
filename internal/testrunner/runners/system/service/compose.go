package service

// DockerComposeRunner knows how to setup and teardown a service defined via
// a docker-compose.yml file.
type DockerComposeRunner struct {
	ymlPath string
}

// NewDockerComposeRunner returns a new instance of a DockerComposeRunner.
func NewDockerComposeRunner(ymlPath string) (*DockerComposeRunner, error) {
	return &DockerComposeRunner{
		ymlPath,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (r *DockerComposeRunner) SetUp() (Ctx, error) {
	// TODO
	return nil, nil
}

// TearDown tears down the service.
func (r *DockerComposeRunner) TearDown(ctxt Ctx) error {
	// TODO
	return nil
}
