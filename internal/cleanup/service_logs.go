// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cleanup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/agentdeployer"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
)

// ServiceLogs function removes service logs from temporary directory in the `~/.elastic-package`.
func ServiceLogs() (string, error) {
	logger.Debug("Clean all service logs")

	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return "", fmt.Errorf("can't find service logs dir: %w", err)
	}

	logger.Debugf("Remove folder content (path: %s)", locationManager.ServiceLogDir())
	err = files.RemoveContent(locationManager.ServiceLogDir())
	if err != nil {
		return "", fmt.Errorf("can't remove content (path: %s): %w", locationManager.ServiceLogDir(), err)
	}

	return locationManager.ServiceLogDir(), nil
}

// ServiceLogsIndependentAgent function removes service logs from temporary directory for independent agents in `~/.elastic-package`.
func ServiceLogsIndependentAgents(profile *profile.Profile, workDir string) (string, error) {
	logger.Debug("Clean all service logs from independent Elastic Agents")

	packageRootPath, err := packages.MustFindPackageRoot(workDir)
	if err != nil {
		return "", fmt.Errorf("locating package root failed: %w", err)
	}

	serviceLogDirGlob := agentdeployer.ServiceLogsDirGlobPackage(profile, packageRootPath)

	folders, err := filepath.Glob(serviceLogDirGlob)
	if err != nil {
		return "", fmt.Errorf("pattern malformed: %w", err)
	}
	for _, f := range folders {
		logger.Debugf("Remove folder (path: %s)", f)
		if err := os.RemoveAll(f); err != nil {
			return "", fmt.Errorf("can't remove folder (path: %s): %w", f, err)
		}
	}

	return serviceLogDirGlob, nil
}
