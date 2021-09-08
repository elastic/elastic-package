package stack

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
)

func dockerComposeLogs(serviceName string, snapshotFile string) ([]byte, error) {
	c, err := compose.NewProject(DockerComposeProjectName, snapshotFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not create docker compose project")
	}

	opts := compose.CommandOptions{
		Services: []string{serviceName},
	}

	out, err := c.Logs(opts)
	if err != nil {
		return nil, errors.Wrap(err, "running command failed")
	}
	return out, nil
}

func dockerInternalLogs(serviceName string) ([]byte, bool, error) {
	return nil, false, nil // TODO
}