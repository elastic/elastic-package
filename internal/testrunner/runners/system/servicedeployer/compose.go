package servicedeployer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/logger"
)

// DockerComposeRunner knows how to setup and teardown a service defined via
// a docker-compose.yml file.
type DockerComposeRunner struct {
	ymlPath string
	project string

	stdout io.WriteCloser
	stderr io.WriteCloser
}

// NewDockerComposeRunner returns a new instance of a DockerComposeRunner.
func NewDockerComposeRunner(ymlPath string) (*DockerComposeRunner, error) {
	return &DockerComposeRunner{
		ymlPath: ymlPath,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (r *DockerComposeRunner) SetUp(ctxt common.MapStr) (common.MapStr, error) {
	logger.Infof("setting up service using docker compose runner")
	project, err := getStrFromCtxt(ctxt, "docker.compose.project")
	if err != nil {
		return ctxt, errors.Wrap(err, "could not get docker compose project")
	}
	r.project = project

	c, err := compose.NewProject(project, r.ymlPath)
	if err != nil {
		return ctxt, errors.Wrap(err, "could not create docker compose project for service")
	}

	tempDirPath, err := getStrFromCtxt(ctxt, "tempdir")
	if err != nil {
		return ctxt, errors.Wrap(err, "could not get temporary folder path")
	}

	outFilePath := filepath.Join(tempDirPath, "stdout")
	outFile, err := os.Create(outFilePath)
	if err != nil {
		return ctxt, errors.Wrap(err, "could not create STDOUT file")
	}
	r.stdout = outFile
	c.SetStdout(r.stdout)
	ctxt.Put("Service.STDOUT", outFilePath)

	errFilePath := filepath.Join(tempDirPath, "stderr")
	errFile, err := os.Create(errFilePath)
	if err != nil {
		return ctxt, errors.Wrap(err, "could not create STDERR file")
	}
	r.stderr = errFile
	c.SetStderr(r.stderr)
	ctxt.Put("Service.STDERR", errFilePath)

	if err := c.Up(nil, nil); err != nil {
		return ctxt, errors.Wrap(err, "could not boot up service using docker compose")
	}

	return ctxt, nil
}

// TearDown tears down the service.
func (r *DockerComposeRunner) TearDown(ctxt common.MapStr) error {
	logger.Infof("tearing down service using docker compose runner")
	c, err := compose.NewProject(r.project, r.ymlPath)
	if err != nil {
		return errors.Wrap(err, "could not create docker compose project for service")
	}

	if err := c.Down(nil, nil); err != nil {
		return errors.Wrap(err, "could not shut down service using docker compose")
	}

	if err := r.stderr.Close(); err != nil {
		return errors.Wrap(err, "could not close STDERR file")
	}

	if err := r.stdout.Close(); err != nil {
		return errors.Wrap(err, "could not close STDOUT file")
	}

	return nil
}

func getStrFromCtxt(ctxt common.MapStr, key string) (string, error) {
	v, err := ctxt.GetValue(key)
	if err != nil {
		return "", errors.Wrapf(err, "could not get key %s from context", key)
	}

	val, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("expected value for key %s be a string, got: %v", key, v)
	}

	return val, nil
}
