package service

type DockerComposeRunner struct {
	ymlPath string
}

func NewDockerComposeRunner(ymlPath string) (*DockerComposeRunner, error) {
	return &DockerComposeRunner{
		ymlPath,
	}, nil
}

func (r *DockerComposeRunner) SetUp() (ctx, error) {
	return nil, nil
}

func (r *DockerComposeRunner) TearDown(ctxt ctx) error {
	return nil
}
