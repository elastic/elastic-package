package reporters

import (
	"fmt"

	"github.com/jedib0t/go-pretty/table"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporter(ConsoleReporter, ReportConsole)
}

const (
	ConsoleReporter testrunner.TestReporter = "console"
)

func ReportConsole(results []testrunner.TestResult) (string, error) {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Package", "Data stream", "Test type", "Test name", "Result", "Time taken"})

	for _, r := range results {
		var result string
		if r.ErrorMsg != "" {
			result = fmt.Sprintf("ERROR: %s", r.ErrorMsg)
		} else if r.FailureMsg != "" {
			result = fmt.Sprintf("FAILURE: %s", r.FailureMsg)
		} else {
			result = "PASS"
		}

		t.AppendRow(table.Row{r.Package, r.DataStream, r.TestType, r.Name, result, r.TimeTaken})
	}

	t.SetStyle(table.StyleRounded)
	return t.Render(), nil
}
