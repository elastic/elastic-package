// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"strings"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/logger"
)

// Status shows the status for each service
func Status(options Options) ([]compose.ServiceStatus, error) {
	servicesStatus, err := dockerComposeStatus(options)
	if err != nil {
		return nil, err
	}

	var services []compose.ServiceStatus
	for _, status := range servicesStatus {
		if strings.Contains(status.Name, readyServicesSuffix) {
			logger.Debugf("Filtering out service: %s", status.Name)
			continue
		}
		services = append(services, status)
	}

	return services, nil
}
