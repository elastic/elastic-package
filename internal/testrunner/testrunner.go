package testrunner

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// TestType represents the various supported test types
type TestType string

// RunFunc method defines main run function of a test runner.
type RunFunc func(testFolderPath string) error

var runners = map[TestType]RunFunc{}

// FindTestFolders finds test folders for the given package and, optionally, test type and datasets
func FindTestFolders(packageRootPath string, testType TestType, datasets []string) ([]string, error) {

	// Expected folder structure:
	// <packageRootPath>/
	//   datasets/
	//     <dataset>/
	//       _dev/
	//         test/
	//           <testType>/

	testTypeGlob := "*"
	if testType != "" {
		testTypeGlob = string(testType)
	}

	datasetsGlob := "*"
	if datasets != nil && len(datasets) > 0 {
		datasetsGlob = "("
		datasetsGlob += strings.Join(datasets, "|")
		datasetsGlob += ")"
	}

	testFoldersGlob := path.Join(packageRootPath, "dataset", datasetsGlob, "_dev", "test", testTypeGlob)
	matches, err := filepath.Glob(testFoldersGlob)
	if err != nil {
		return nil, errors.Wrap(err, "error finding test folders")
	}

	return matches, nil
}

// RegisterRunner method registers the test runner.
func RegisterRunner(testType TestType, runFunc RunFunc) {
	runners[testType] = runFunc
}

// Run method delegates execution to the registered test runner, based on the test type.
func Run(testType TestType, testFolderPath string) error {
	runFunc, defined := runners[testType]
	if !defined {
		return fmt.Errorf("unregistered runner test: %s", testType)
	}
	return runFunc(testFolderPath)
}

// TestTypes method returns registered test types.
func TestTypes() []TestType {
	var testTypes []TestType
	for t := range runners {
		testTypes = append(testTypes, t)
	}
	return testTypes
}
