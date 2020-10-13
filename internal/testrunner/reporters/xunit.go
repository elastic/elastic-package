package reporters

import (
	"encoding/xml"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporter(XUnitReporter, ReportXUnit)
}

const (
	XUnitReporter testrunner.TestReporter = "xUnit"
)

type testSuites struct {
	XMLName xml.Name    `xml:"testsuites"`
	Suites  []testSuite `xml:"testsuite"`
}
type testSuite struct {
	Suites []testSuite `xml:"testsuite"`
	Cases  []testCase  `xml:"testcase"`
}
type testCase struct {
	Name string        `xml:"Name,attr"`
	Time time.Duration `xml:"Time,attr"`

	Error   string `xml:"Error"`
	Failure string `xml:"Failure"`
}

func ReportXUnit(results []testrunner.TestResult) (string, error) {
	// package => data stream => test type => test cases
	packages := map[string]map[string]map[testrunner.TestType][]testCase{}

	var numPackages int

	for _, r := range results {
		if _, exists := packages[r.Package]; !exists {
			packages[r.Package] = map[string]map[testrunner.TestType][]testCase{}
			numPackages++
		}

		if _, exists := packages[r.Package][r.DataStream]; !exists {
			packages[r.Package][r.DataStream] = map[testrunner.TestType][]testCase{}
		}

		if packages[r.Package][r.DataStream][r.TestType] == nil {
			packages[r.Package][r.DataStream][r.TestType] = make([]testCase, 1)
		}

		c := testCase{
			Name:    r.Name,
			Time:    r.TimeTaken,
			Error:   r.ErrorMsg,
			Failure: r.FailureMsg,
		}

		packages[r.Package][r.DataStream][r.TestType] = append(packages[r.Package][r.DataStream][r.TestType], c)
	}

	var ts testSuites
	ts.Suites = make([]testSuite, numPackages)

	for _, pkg := range packages {
		pkgSuite := testSuite{
			Suites: make([]testSuite, 1),
		}

		for _, ds := range pkg {
			dsSuite := testSuite{
				Suites: make([]testSuite, 1),
			}

			for _, cases := range ds {
				testTypeSuite := testSuite{
					Cases: cases,
				}
				dsSuite.Suites = append(dsSuite.Suites, testTypeSuite)
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
