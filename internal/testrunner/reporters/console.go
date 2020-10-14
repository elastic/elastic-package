package reporters

import (
	"fmt"

	"github.com/jedib0t/go-pretty/table"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporter(ConsoleReporter, reportConsole)
}

const (
	// ConsoleReporter reports test results in a console-friendly tabular format
	ConsoleReporter testrunner.TestReporter = "console"
)

func reportConsole(results []testrunner.TestResult) (string, error) {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Package", "Data stream", "Test name", "Result", "Time elapsed"})

	for _, r := range results {
		var result string
		if r.ErrorMsg != "" {
			result = fmt.Sprintf("ERROR: %s", r.ErrorMsg)
		} else if r.FailureMsg != "" {
			result = fmt.Sprintf("FAIL: %s", r.FailureMsg)
		} else {
			result = "PASS"
		}

		t.AppendRow(table.Row{r.Package, r.DataStream, r.Name, result, r.TimeElapsed})
	}

	t.SetStyle(table.StyleRounded)
	return t.Render(), nil
}
