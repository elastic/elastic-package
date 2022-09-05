// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchrunner

import (
	"fmt"
)

// BenchmarkResult represents the result of a benchmark run.
// This is modeled after the xUnit benchmark schema.
// See https://github.com/Autodesk/jenkinsci-benchmark-plugin/blob/master/doc/EXAMPLE_SCHEMA_XML_DEFAULT.md
type BenchmarkResult struct {
	// XMLName is a zero-length field used as an annotation for XML marshaling.
	XMLName struct{} `xml:"group"`

	// Name of this benchmark run.
	Name string `xml:"name,attr"`

	// Description of the benchmark run.
	Description string `xml:"description,omitempty"`

	// Parameters used for this benchmark.
	Parameters []BenchmarkValue `xml:"parameter"`

	// Tests holds the results for the benchmark.
	Tests []BenchmarkTest `xml:"test"`
}

// BenchmarkTest models a particular test performed during a benchmark.
type BenchmarkTest struct {
	// Name of this test.
	Name string `xml:"name,attr"`
	// Detailed benchmark tests will be printed to the output but not
	// included in xUnit reports.
	Detailed bool `xml:"-"`
	// Description of this test.
	Description string `xml:"description,omitempty"`
	// Parameters for this test.
	Parameters []BenchmarkValue `xml:"parameter"`
	// Results of the test.
	Results []BenchmarkValue `xml:"result"`
}

// BenchmarkValue represents a value (result or parameter)
// with an optional associated unit.
type BenchmarkValue struct {
	// Name of the value.
	Name string `xml:"name,attr"`

	// Description of the value.
	Description string `xml:"description,omitempty"`

	// Unit used for this value.
	Unit string `xml:"unit,omitempty"`

	// Value is of any type, usually string or numeric.
	Value interface{} `xml:"value,omitempty"`
}

// PrettyValue returns a BenchmarkValue's value nicely-formatted.
func (p BenchmarkValue) PrettyValue() (r string) {
	if str, ok := p.Value.(fmt.Stringer); ok {
		return str.String()
	}
	if float, ok := p.Value.(float64); ok {
		r = fmt.Sprintf("%.02f", float)
	} else {
		r = fmt.Sprintf("%v", p.Value)
	}
	if p.Unit != "" {
		r += p.Unit
	}
	return r
}
