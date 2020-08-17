package pipeline

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining pipeline tests
	TestType testrunner.TestType = "pipeline"
)

type runner struct {
	testFolderPath string
}

// Run runs the pipeline tests defined under the given folder
func Run(testFolderPath string) error {
	r := runner{testFolderPath}
	return r.run()
}

func (r *runner) run() error {
	fmt.Println("pipeline run", r.testFolderPath)
	return nil
}

func init() {
	testrunner.RegisterRunner(TestType, Run)
}
