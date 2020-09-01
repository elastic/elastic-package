package servicedeployer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	stdoutFileName = "stdout"
	stderrFileName = "stderr"
)

// DockerComposeRunner knows how to setup and teardown a service defined via
// a docker-compose.yml file.
type DockerComposeRunner struct {
	ymlPath      string
	project      string
	stackNetwork string

	stdout io.WriteCloser
	stderr io.WriteCloser

	stdoutFilePath string
	stderrFilePath string
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
	r.project = "elastic-package-service"

	c, err := compose.NewProject(r.project, r.ymlPath)
	if err != nil {
		return ctxt, errors.Wrap(err, "could not create docker compose project for service")
	}

	// Boot up service
	opts := compose.CommandOptions{
		ExtraArgs: []string{"-d"},
	}
	if err := c.Up(opts); err != nil {
		return ctxt, errors.Wrap(err, "could not boot up service using docker compose")
	}

	// Build service container name
	serviceName, err := getStrFromCtxt(ctxt, "service.name")
	if err != nil {
		return ctxt, errors.Wrap(err, "could not get service name")
	}
	serviceContainer := fmt.Sprintf("%s_%s_1", r.project, serviceName)

	// Redirect service container's STDOUT and STDERR streams to files in local logs folder
	localLogsFolder, err := getStrFromCtxt(ctxt, "service.logs.folder.local")
	if err != nil {
		return ctxt, errors.Wrap(err, "could not get service logs folder path on local filesystem")
	}

	agentLogsFolder, err := getStrFromCtxt(ctxt, "service.logs.folder.agent")
	if err != nil {
		return ctxt, errors.Wrap(err, "could not get service logs folder path on agent container filesystem")
	}

	r.stdoutFilePath = filepath.Join(localLogsFolder, stdoutFileName)
	logger.Debugf("creating temp file %s to hold service container %s STDOUT", r.stdoutFilePath, serviceContainer)
	outFile, err := os.Create(r.stdoutFilePath)
	if err != nil {
		return ctxt, errors.Wrap(err, "could not create STDOUT file")
	}
	r.stdout = outFile
	ctxt.Put("Service.STDOUT", agentLogsFolder+stdoutFileName)

	r.stderrFilePath = filepath.Join(localLogsFolder, stderrFileName)
	logger.Debugf("creating temp file %s to hold service container %s STDERR", r.stderrFilePath, serviceContainer)
	errFile, err := os.Create(r.stderrFilePath)
	if err != nil {
		return ctxt, errors.Wrap(err, "could not create STDERR file")
	}
	r.stderr = errFile
	ctxt.Put("Service.STDERR", agentLogsFolder+stderrFileName)

	logger.Debugf("redirecting service container %s STDOUT and STDERR to temp files", serviceContainer)
	cmd := exec.Command("docker", "attach", "--no-stdin", serviceContainer)
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr

	if err := cmd.Start(); err != nil {
		return ctxt, errors.Wrap(err, "could not redirect service container STDOUT and STDERR streams")
	}

	logger.Debugf("attaching service container %s to stack network %s", serviceContainer, r.stackNetwork)
	r.stackNetwork = fmt.Sprintf("%s_default", stack.DockerComposeProjectName)

	cmd = exec.Command("docker", "network", "connect", r.stackNetwork, serviceContainer)
	if err := cmd.Run(); err != nil {
		return ctxt, errors.Wrap(err, "could not attach service container to stack network")
	}

	logger.Debugf("adding service container %s internal ports to context", serviceContainer)
	serviceComposeConfig, err := c.Config(compose.CommandOptions{})
	if err != nil {
		return ctxt, errors.Wrap(err, "could not get Docker Compose configuration for service")
	}

	service := serviceComposeConfig.Services[serviceName]
	for idx, port := range service.Ports {
		ctxt.Put(fmt.Sprintf("Service.Ports.%d", idx), port)

		// Special case for convenience: assume first port is the main port
		if idx == 0 {
			ctxt.Put("Service.Port", port)
		}
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

	opts := compose.CommandOptions{}
	if err := c.Down(opts); err != nil {
		return errors.Wrap(err, "could not shut down service using docker compose")
	}

	if err := r.stderr.Close(); err != nil {
		return errors.Wrapf(err, "could not close STDERR file: %s", r.stderrFilePath)
	}
	if err := os.Remove(r.stderrFilePath); err != nil {
		return errors.Wrapf(err, "could not delete STDERR file: %s", r.stderrFilePath)
	}

	if err := r.stdout.Close(); err != nil {
		return errors.Wrap(err, "could not close STDOUT file")
	}
	if err := os.Remove(r.stdoutFilePath); err != nil {
		return errors.Wrapf(err, "could not delete STDOUT file: %s", r.stdoutFilePath)
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
