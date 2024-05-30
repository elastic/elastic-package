// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"sort"
	"strings"
)

// Status shows the status for each service
func Status(ctx context.Context, options Options) ([]ServiceStatus, error) {
	dockerCompose := newDockerCompose(dockerComposeOptions{
		Logger:  options.Logger,
		Profile: options.Profile,
	})

	servicesStatus, err := dockerCompose.Status(ctx)
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
