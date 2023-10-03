// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	elasticAgentService = "elastic-agent"
	fleetServerService  = "fleet-server"
)

// DumpOptions defines dumping options for Elatic stack data.
type DumpOptions struct {
	Output  string
	Profile *profile.Profile
}

// Dump function exports stack data and dumps them as local artifacts, which can be used for debug purposes.
func Dump(options DumpOptions) (string, error) {
	logger.Debugf("Dump Elastic stack data")

	err := dumpStackLogs(options)
	if err != nil {
		return "", fmt.Errorf("can't dump Elastic stack logs: %w", err)
	}
	return options.Output, nil
}

func dumpStackLogs(options DumpOptions) error {
	logger.Debugf("Dump stack logs (location: %s)", options.Output)
	err := os.RemoveAll(options.Output)
	if err != nil {
		return fmt.Errorf("can't remove output location: %w", err)
	}

	logsPath := filepath.Join(options.Output, "logs")
	err = os.MkdirAll(logsPath, 0755)
	if err != nil {
		return fmt.Errorf("can't create output location (path: %s): %w", logsPath, err)
	}

	services, err := localServiceNames(DockerComposeProjectName(options.Profile))
	if err != nil {
		return fmt.Errorf("failed to get local services: %w", err)
	}

	for _, serviceName := range services {
		logger.Debugf("Dump stack logs for %s", serviceName)

		content, err := dockerComposeLogs(serviceName, options.Profile)
		if err != nil {
			logger.Errorf("can't fetch service logs (service: %s): %v", serviceName, err)
		} else {
			writeLogFiles(logsPath, serviceName, content)
		}

		err = copyDockerInternalLogs(serviceName, logsPath, options.Profile)
		if err != nil {
			logger.Errorf("can't copy internal logs (service: %s): %v", serviceName, err)
		}
	}
	return nil
}

func writeLogFiles(logsPath, serviceName string, content []byte) {
	err := os.WriteFile(filepath.Join(logsPath, fmt.Sprintf("%s.log", serviceName)), content, 0644)
	if err != nil {
		logger.Errorf("can't write service logs (service: %s): %v", serviceName, err)
	}
}

// DumpLogsFile returns the file path to the logs of a given service
func DumpLogsFile(options DumpOptions, serviceName string) string {
	return filepath.Join(options.Output, "logs", fmt.Sprintf("%s.log", serviceName))
}
