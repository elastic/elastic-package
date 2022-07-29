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
