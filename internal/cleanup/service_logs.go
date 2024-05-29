// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cleanup

import (
	"fmt"
	"log/slog"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/files"
)

// ServiceLogs function removes service logs from temporary directory in the `~/.elastic-package`.
func ServiceLogs(logger *slog.Logger) (string, error) {
	logger.Debug("Clean all service logs")

	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return "", fmt.Errorf("can't find service logs dir: %w", err)
	}

	logger.Debug("Remove folder content (path: %s)", slog.String("path", locationManager.ServiceLogDir()))
	err = files.RemoveContent(locationManager.ServiceLogDir())
	if err != nil {
		return "", fmt.Errorf("can't remove content (path: %s): %w", locationManager.ServiceLogDir(), err)
	}
	return locationManager.ServiceLogDir(), nil
}
