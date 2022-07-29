// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
)

// Status shows the status for each service
func Status(options Options) ([]compose.ServiceStatus, error) {
	opts := options
	opts.Services = observedServices

	statusServices, err := dockerComposeStatus(opts)
	if err != nil {
		return nil, errors.Wrap(err, "stack status failed")
	}

	return statusServices, nil
}
