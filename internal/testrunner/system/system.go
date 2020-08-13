package system

import (
	"os"
	"path"

	"github.com/pkg/errors"
)

type Options struct {
	FailOnMissing bool
}

// Run runs the system tests for the given package.
func Run(packageRootPath string, options Options) error {
	systemTestsPath, err := findSystemTestsPath(packageRootPath)
	if err != nil && err == ErrNoSystemTests && !options.FailOnMissing {
		return nil
	}

	if err != nil {
		return err
	}

	runner, err := newRunner(systemTestsPath)
	if err != nil {
		return errors.Wrap(err, "could not instantiate system tests runner")
	}

	if err := runner.run(); err != nil {
		return errors.Wrap(err, "system tests failed")
	}

	return nil
}

func findSystemTestsPath(packageRootPath string) (string, error) {
	systemTestsPath := path.Join(packageRootPath, "_dev", "test", "system")
	info, err := os.Stat(systemTestsPath)
	if err != nil && os.IsNotExist(err) {
		return "", ErrNoSystemTests
	}
	if err != nil {
		return "", ErrNoSystemTests
	}

	if !info.IsDir() {
		return "", errors.New("system tests path is not a folder")
	}

	return systemTestsPath, nil
}
