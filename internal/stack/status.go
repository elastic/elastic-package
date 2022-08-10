// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"github.com/elastic/elastic-package/internal/compose"
)

// Status shows the status for each service
func Status(options Options) ([]compose.ServiceStatus, error) {
	servicesStatus, err := dockerComposeStatus(options)
	if err != nil {
		return nil, err
	}

	return servicesStatus, nil
}
