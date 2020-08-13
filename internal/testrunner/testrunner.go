package testrunner

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// TestType represents the various supported test types
type TestType string

const (
	// TestTypeSystem represents system tests
	TestTypeSystem TestType = "system"
)

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
