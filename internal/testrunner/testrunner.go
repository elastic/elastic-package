// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/pkg/errors"
)

// TestType represents the various supported test types
type TestType string

// TestOptions contains test runner options.
type TestOptions struct {
	TestFolder         TestFolder
	PackageRootPath    string
	GenerateTestResult bool
	ESClient           *elasticsearch.Client
}

// RunFunc method defines main run function of a test runner.
type RunFunc func(options TestOptions) error

var runners = map[TestType]RunFunc{}

// TestFolder encapsulates the test folder path and names of the package + dataset
// to which the test folder belongs.
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

	var paths []string
	if datasets != nil && len(datasets) > 0 {
		sort.Strings(datasets)
		for _, dataset := range datasets {
			p, err := findTestFolderPaths(packageRootPath, dataset, testTypeGlob)
			if err != nil {
				return nil, err
			}

			paths = append(paths, p...)
		}
	} else {
		p, err := findTestFolderPaths(packageRootPath, "*", testTypeGlob)
		if err != nil {
			return nil, err
		}

		paths = p
	}

	folders := make([]TestFolder, len(paths))
	_, pkg := filepath.Split(packageRootPath)
	for idx, p := range paths {
		relP := strings.TrimPrefix(p, packageRootPath)
		parts := strings.Split(relP, string(filepath.Separator))
		dataset := parts[2]

		folder := TestFolder{
			p,
			pkg,
			dataset,
		}

		folders[idx] = folder
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

func findTestFolderPaths(packageRootPath, datasetGlob, testTypeGlob string) ([]string, error) {
	testFoldersGlob := filepath.Join(packageRootPath, "dataset", datasetGlob, "_dev", "test", testTypeGlob)
	paths, err := filepath.Glob(testFoldersGlob)
	if err != nil {
		return nil, errors.Wrap(err, "error finding test folders")
	}

	return paths, err
}
