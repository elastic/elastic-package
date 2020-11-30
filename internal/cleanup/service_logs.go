// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cleanup

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
)

// ServiceLogs function removes service logs from temporary directory in the `~/.elastic-package`.
func ServiceLogs() (string, error) {
	logger.Debug("Clean all service logs")

	serviceLogsDir, err := install.ServiceLogsDir()
	if err != nil {
		return "", errors.Wrap(err, "can't find service logs dir")
	}

	logger.Debugf("Remove folder content (path: %s)", serviceLogsDir)
	err = files.RemoveContent(serviceLogsDir)
	if err != nil {
		return "", errors.Wrapf(err, "can't remove content (path: %s)", serviceLogsDir)
	}
	return serviceLogsDir, nil
}
