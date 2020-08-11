package system

import (
	"fmt"
	"os"
	"path"

	"github.com/pkg/errors"
)

// Test runner for system tests.

func Run(packageRootPath string) error {
	systemTestsPath, err := findSystemTestsPath(packageRootPath)
	if err != nil {
		return err
	}

	fmt.Printf("system tests found in [%s]!\n", systemTestsPath)
	return nil
}

func findSystemTestsPath(packageRootPath string) (string, error) {
	systemTestsPath := path.Join(packageRootPath, "_dev", "test", "system")
	info, err := os.Stat(systemTestsPath)
	if err != nil && os.IsNotExist(err) {
		return "", errors.Wrap(err, "package does not have system tests folder defined")
	}
	if err != nil {
		return "", errors.Wrap(err, "error finding system tests folder")
	}

	if !info.IsDir() {
		return "", errors.New("system tests path is not a folder")
	}

	return systemTestsPath, nil
}
