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
func Run(testFolderPath string) error {
	r := runner{testFolderPath}
	return r.run()
}

func (r *runner) run() error {
	fmt.Println("system run", r.testFolderPath)
	return nil
}

func init() {
	testrunner.RegisterRunner(TestType, Run)
}
