// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

//go:build ignore
// +build ignore

// The j2xunit command consumes JSON-formatted test output from the go
// test command and emits xunit-compatible XML output.
package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {
	dec := json.NewDecoder(os.Stdin)
	suite := make(map[string][]test)
	for {
		var t test
		err := dec.Decode(&t)
		if err != nil {
			if err != io.EOF {
				log.Fatalf("error reading test data stream: %v", err)
			}
			break
		}
		suite[t.Package] = append(suite[t.Package], t)
	}
	var ju junit
	for pkg, tests := range suite {
		var (
			run, fail, skip int
			results         []test
		)
		output := make(map[string][]string)
		for _, t := range tests {
			if t.Test == "" {
				continue
			}
			switch t.Action {
			case "output":
				k := t.Package + "·" + t.Test
				output[k] = append(output[k], t.Output)
			case "run":
				run++
			case "pass":
			case "fail":
				fail++
				t.Failure = &failure{
					Type:    "go.error",
					Message: "error",
					Output:  strings.Join(output[t.Package+"·"+t.Test], ""),
				}
			case "skip":
				skip++
				t.Skipped = true
			}

			if t.Action != "fail" {
				t.Output = ""
			}
			if t.Action == "pass" || t.Action == "fail" || t.Action == "skip" {
				results = append(results, t)
			}
		}
		sort.Slice(results, func(i, j int) bool { return results[i].Test < results[j].Test })
		ju.Testsuite = append(ju.Testsuite, testPackage{Name: pkg, Tests: run, Failures: fail, Skip: skip, Testcase: results})
	}
	sort.Slice(ju.Testsuite, func(i, j int) bool { return ju.Testsuite[i].Name < ju.Testsuite[j].Name })
	b, err := xml.MarshalIndent(ju, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(xml.Header)
	os.Stdout.Write(b)
}

type junit struct {
	XMLName   xml.Name      `xml:"testsuites"`
	Testsuite []testPackage `xml:"testsuite"`
}

type testPackage struct {
	Text     string `xml:",chardata"`
	Name     string `xml:"name,attr"`
	Tests    int    `xml:"tests,attr"`
	Errors   int    `xml:"errors,attr"`
	Failures int    `xml:"failures,attr"`
	Skip     int    `xml:"skip,attr"`
	Testcase []test `xml:"testcase"`
}

type test struct {
	Time    time.Time `xml:"-"`
	Action  string    `xml:"-"`
	Output  string    `json:"Output" xml:"-"`
	Package string    `json:"Package" xml:"classname,attr"`
	Test    string    `json:"Test" xml:"name,attr"`
	Elapsed float64   `json:"Elapsed" xml:"time,attr"`
	Skipped skipped   `xml:"skipped"`
	Failure *failure  `xml:"failure"`
}

type failure struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
	Output  string `xml:",cdata"`
}

type skipped bool

func (s skipped) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	var err error
	if s {
		err = e.EncodeToken(start)
		if err != nil {
			return err
		}
		err = e.EncodeToken(start.End())
	}
	return err
}
