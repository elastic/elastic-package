package stack

import "github.com/pkg/errors"

// Update pulls down the most recent versions of the Docker images
func Update(options Options) error {
	err := dockerComposePull(options)
	if err != nil {
		return errors.Wrap(err, "updating docker images failed")
	}
	return nil
}