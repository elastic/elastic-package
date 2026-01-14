// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/profile"
)

const (
	serviceLogsDir = "service_logs"
	deployerDir    = "deployer"
)

func ServiceLogsDir(profile *profile.Profile) string {
	return filepath.Join(profile.ProfilePath, serviceLogsDir)
}

func ServiceLogsDirGlobPackage(profile *profile.Profile, packageRoot string) string {
	packageFolderName := filepath.Base(packageRoot)
	return fmt.Sprintf("%s/agent-%s-*", ServiceLogsDir(profile), packageFolderName)
}

func CreateServiceLogsDir(profile *profile.Profile, packageRootpath, dataStream, runID string) (string, error) {
	packageFolderName := filepath.Base(packageRootpath)
	folderName := fmt.Sprintf("agent-%s", packageFolderName)
	if dataStream != "" {
		folderName = fmt.Sprintf("%s-%s", folderName, dataStream)
	}
	folderName = fmt.Sprintf("%s-%s", folderName, runID)

	dirPath := filepath.Join(ServiceLogsDir(profile), folderName)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return "", fmt.Errorf("mkdir failed for service logs (path: %s): %w", dirPath, err)
	}
	return dirPath, nil
}

func CreateDeployerDir(profile *profile.Profile, name string) (string, error) {
	customAgentDir := filepath.Join(profile.ProfilePath, deployerDir, name)
	err := os.MkdirAll(customAgentDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory for custom agent files: %w", err)
	}
	return customAgentDir, nil
}
