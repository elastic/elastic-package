// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"sort"
	"strings"
)

// Status shows the status for each service
func Status(options Options) ([]ServiceStatus, error) {
	servicesStatus, err := dockerComposeStatus(options)
	if err != nil {
		return nil, err
	}

	var services []ServiceStatus
	for _, status := range servicesStatus {
		if strings.Contains(status.Name, readyServicesSuffix) {
			continue
		}
		services = append(services, status)
	}

	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })

	return services, nil
}
