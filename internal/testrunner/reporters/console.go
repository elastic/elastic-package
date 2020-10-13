package reporters

import (
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporter(ConsoleReporter, ReportConsole)
}

const (
	ConsoleReporter testrunner.TestReporter = "console"
)

func ReportConsole(results []testrunner.TestResult) (string, error) {
	var sb strings.Builder
	for _, r := range results {
		var result string
		if r.ErrorMsg != "" {
			result = fmt.Sprintf("ERROR: %s", r.ErrorMsg)
		} else if r.FailureMsg != "" {
			result = fmt.Sprintf("FAILURE: %s", r.FailureMsg)
		} else {
			result = "PASS"
		}

		sb.WriteString(fmt.Sprintf(
			"[%s/%s] - %s test - %s (took %d) - %s\n",
			r.Package, r.DataStream,
			r.TestType,
			r.Name,
			r.TimeTaken,
			result,
		))
	}

	return sb.String(), nil
}
