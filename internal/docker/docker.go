// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docker

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

// Pull downloads the latest available revision of the image.
func Pull(image string) error {
	cmd := exec.Command("docker", "pull", image)

	if logger.IsDebugMode() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	logger.Debugf("running command: %s", cmd)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running docker command failed")
	}
	return nil
}
