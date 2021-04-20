// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

var observedServices = []string{"elasticsearch", "elastic-agent", "fleet-server", "kibana", "package-registry"}

// DumpOptions defines dumping options for Elatic stack data.
type DumpOptions struct {
	Output  string
	Profile profile.ConfigProfile
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
	logger.Debugf("Dump stack logs")

	logger.Debugf("Recreate the output location (path: %s)", options.Output)
	err := os.RemoveAll(options.Output)
	if err != nil {
		return errors.Wrap(err, "can't remove output location")
	}

	logsPath := filepath.Join(options.Output, "logs")
	err = os.MkdirAll(logsPath, 0755)
	if err != nil {
		return errors.Wrap(err, "can't create output location")
	}

	snapshotPath := options.Profile.Fetch(profile.SnapshotFile)

	for _, serviceName := range observedServices {
		logger.Debugf("Dump stack logs for %s", serviceName)

		serviceLogs, err := dockerComposeLogs(serviceName, snapshotPath)
		if err != nil {
			logger.Errorf("can't fetch service logs (service: %s): %v", serviceName, err)
			continue
		}

		err = ioutil.WriteFile(filepath.Join(logsPath, fmt.Sprintf("%s.log", serviceName)), serviceLogs, 0644)
		if err != nil {
			logger.Errorf("can't write service logs (service: %s): %v", serviceName, err)
			continue
		}
	}
	return nil
}
