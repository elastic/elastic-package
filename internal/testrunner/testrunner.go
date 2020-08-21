// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// TestType represents the various supported test types
type TestType string

// TestOptions contains test runner options.
type TestOptions struct {
	TestFolder         TestFolder
	PackageRootPath    string
	GenerateTestResult bool
}

// RunFunc method defines main run function of a test runner.
type RunFunc func(options TestOptions) error

var runners = map[TestType]RunFunc{}

type TestFolder struct {
	Path    string
	Package string
	Dataset string
}

// FindTestFolders finds test folders for the given package and, optionally, test type and datasets
func FindTestFolders(packageRootPath string, testType TestType, datasets []string) ([]TestFolder, error) {

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

	testFoldersGlob := filepath.Join(packageRootPath, "dataset", datasetsGlob, "_dev", "test", testTypeGlob)
	paths, err := filepath.Glob(testFoldersGlob)
	if err != nil {
		return nil, errors.Wrap(err, "error finding test folders")
	}

	folders := make([]TestFolder, len(matches))
	for _, p := range paths {
		parts := filepath.SplitList(p)
		pkg := parts[0]
		dataset := parts[2]

		folder := TestFolder{
			p,
			pkg,
			dataset,
		}

		folders = append(folders, folder)
	}

	return folders, nil
}

// RegisterRunner method registers the test runner.
func RegisterRunner(testType TestType, runFunc RunFunc) {
	runners[testType] = runFunc
}

// Run method delegates execution to the registered test runner, based on the test type.
func Run(testType TestType, options TestOptions) error {
	runFunc, defined := runners[testType]
	if !defined {
		return fmt.Errorf("unregistered runner test: %s", testType)
	}
	return runFunc(options)
}

// TestTypes method returns registered test types.
func TestTypes() []TestType {
	var testTypes []TestType
	for t := range runners {
		testTypes = append(testTypes, t)
	}
	return testTypes
}
