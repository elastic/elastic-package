// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	elasticAgentService = "elastic-agent"
	fleetServerService  = "fleet-server"
)

var observedServices = []string{"elasticsearch", elasticAgentService, fleetServerService, "kibana", "package-registry"}

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
		return "", errors.Wrap(err, "can't dump Elastic stack logs")
	}
	return options.Output, nil
}

func dumpStackLogs(options DumpOptions) error {
	logger.Debugf("Dump stack logs (location: %s)", options.Output)
	err := os.RemoveAll(options.Output)
	if err != nil {
		return errors.Wrap(err, "can't remove output location")
	}

	logsPath := filepath.Join(options.Output, "logs")
	err = os.MkdirAll(logsPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "can't create output location (path: %s)", logsPath)
	}

	snapshotPath := options.Profile.FetchPath(profile.SnapshotFile)

	for _, serviceName := range observedServices {
		logger.Debugf("Dump stack logs for %s", serviceName)

		content, err := dockerComposeLogs(serviceName, snapshotPath)
		if err != nil {
			logger.Errorf("can't fetch service logs (service: %s): %v", serviceName, err)
		} else {
			writeLogFiles(logsPath, serviceName, content)
		}

		err = copyDockerInternalLogs(serviceName, logsPath)
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
