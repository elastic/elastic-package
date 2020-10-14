package formats

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporterFormat(ReportFormatHuman, reportHumanFormat)
}

const (
	// ReportFormatHuman reports test results in a human-readable format
	ReportFormatHuman testrunner.TestReportFormat = "human"
)

func reportHumanFormat(results []testrunner.TestResult) (string, error) {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Package", "Data stream", "Test type", "Test name", "Result", "Time elapsed"})

	for _, r := range results {
		var result string
		if r.ErrorMsg != "" {
			result = fmt.Sprintf("ERROR: %s", r.ErrorMsg)
		} else if r.FailureMsg != "" {
			result = fmt.Sprintf("FAIL: %s", r.FailureMsg)
		} else {
			result = "PASS"
		}

		t.AppendRow(table.Row{r.Package, r.DataStream, r.TestType, r.Name, result, r.TimeElapsed})
	}

	t.SetStyle(table.StyleRounded)

	s := t.Render()

	var details []string
	for _, r := range results {
		if r.FailureMsg == "" {
			continue
		}

		detail := fmt.Sprintf("%s/%s %s:\n%s", r.Package, r.DataStream, r.Name, r.FailureDetails)
		details = append(details, detail)
	}

	if len(details) > 0 {
		s += "\n\nFAILURE DETAILS:\n\n" + strings.Join(details, "\n")
	}

	return s, nil
}
