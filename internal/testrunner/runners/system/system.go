// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining system tests
	TestType testrunner.TestType = "system"
)

type runner struct {
	testFolderPath string
}

// Run runs the system tests defined under the given folder
func Run(options testrunner.TestOptions) error {
	r := runner{options.TestFolderPath}
	return r.run()
}

func (r *runner) run() error {
	fmt.Println("system run", r.testFolderPath)
	return nil
}

func init() {
	testrunner.RegisterRunner(TestType, Run)
}
