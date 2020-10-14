package outputs

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporterOutput(ReportOutputSTDOUT, reportToSTDOUT)
}

const (
	// ReportOutputSTDOUT reports test results to STDOUT
	ReportOutputSTDOUT testrunner.TestReportOutput = "stdout"
)

func reportToSTDOUT(report string, _ testrunner.TestReportFormat) error {
	fmt.Println("--- Test results: START ---")
	fmt.Println(report)
	fmt.Println("--- Test results: END ---")
	fmt.Println("Done")

	return nil
}
