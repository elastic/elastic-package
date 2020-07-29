package files

import (
	"os"

	"github.com/pkg/errors"
)

// ClearDir method removes all items from the destination directory.
// Internally it deletes and recreates the directory.
func ClearDir(destinationPath string) error {
	err := os.RemoveAll(destinationPath)
	if err != nil {
		return errors.Wrapf(err, "removing directory failed (path: %s)", destinationPath)
	}

	err = os.MkdirAll(destinationPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", destinationPath)
	}
	return nil
}
