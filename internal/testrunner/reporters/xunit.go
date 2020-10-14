package reporters

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporter(XUnitReporter, reportXUnit)
}

const (
	// XUnitReporter reports test results in the xUnit format
	XUnitReporter testrunner.TestReporter = "xUnit"
)

type testSuites struct {
	XMLName xml.Name    `xml:"testsuites"`
	Suites  []testSuite `xml:"testsuite"`
}
type testSuite struct {
	Name    string      `xml:"name,attr"`
	Comment string      `xml:",comment"`
	Suites  []testSuite `xml:"testsuite,omitempty"`
	Cases   []testCase  `xml:"testcase,omitempty"`
}
type testCase struct {
	Name string        `xml:"name,attr"`
	Time time.Duration `xml:"time,attr"`

	Error   string `xml:"error,omitempty"`
	Failure string `xml:"failure,omitempty"`
}

func reportXUnit(results []testrunner.TestResult) (string, error) {
	// package => data stream => test cases
	packages := map[string]map[string][]testCase{}

	var numPackages int

	for _, r := range results {
		if _, exists := packages[r.Package]; !exists {
			packages[r.Package] = map[string][]testCase{}
			numPackages++
		}

		if _, exists := packages[r.Package][r.DataStream]; !exists {
			packages[r.Package][r.DataStream] = make([]testCase, 0)
		}

		c := testCase{
			Name:    r.Name,
			Time:    r.TimeTaken,
			Error:   r.ErrorMsg,
			Failure: r.FailureMsg,
		}

		packages[r.Package][r.DataStream] = append(packages[r.Package][r.DataStream], c)
	}

	var ts testSuites
	ts.Suites = make([]testSuite, 0)

	for pkgName, pkg := range packages {
		pkgSuite := testSuite{
			Name:    pkgName,
			Comment: fmt.Sprintf("test suite for package: %s", pkgName),
			Suites:  make([]testSuite, 0),
		}

		for dsName, ds := range pkg {
			dsSuite := testSuite{
				Name:    dsName,
				Comment: fmt.Sprintf("test suite for data stream: %s", dsName),
				Cases:   ds,
			}

			pkgSuite.Suites = append(pkgSuite.Suites, dsSuite)
		}

		ts.Suites = append(ts.Suites, pkgSuite)
	}

	out, err := xml.MarshalIndent(&ts, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "unable to format test results as xUnit")
	}

	return xml.Header + string(out), nil
}
