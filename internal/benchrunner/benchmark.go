// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchrunner

import (
	"fmt"
)

type BenchmarkResult struct {
	// XMLName is a zero-length field used as an annotation for XML marshaling.
	XMLName struct{} `xml:"group" json:"-"`
	// Type of benchmark
	Type string `xml:"type" json:"type"`
	// Package of the benchmark
	Package string `xml:"package" json:"package"`
	// DataStream of the benchmark
	DataStream string `xml:"data_stream" json:"data_stream"`
	// Description of the benchmark run.
	Description string `xml:"description,omitempty" json:"description,omitempty"`
	// Parameters used for this benchmark.
	Parameters []BenchmarkValue `xml:"parameters,omitempty" json:"parameters,omitempty"`
	// Tests holds the results for the benchmark.
	Tests []BenchmarkTest `xml:"test" json:"test"`
}

// BenchmarkTest models a particular test performed during a benchmark.
type BenchmarkTest struct {
	// Name of this test.
	Name string `xml:"name" json:"name"`
	// Detailed benchmark tests will be printed to the output but not
	// included in file reports.
	Detailed bool `xml:"-" json:"-"`
	// Description of this test.
	Description string `xml:"description,omitempty" json:"description,omitempty"`
	// Parameters for this test.
	Parameters []BenchmarkValue `xml:"parameters,omitempty" json:"parameters,omitempty"`
	// Results of the test.
	Results []BenchmarkValue `xml:"result" json:"result"`
}

// BenchmarkValue represents a value (result or parameter)
// with an optional associated unit.
type BenchmarkValue struct {
	// Name of the value.
	Name string `xml:"name" json:"name"`
	// Description of the value.
	Description string `xml:"description,omitempty" json:"description,omitempty"`
	// Unit used for this value.
	Unit string `xml:"unit,omitempty" json:"unit,omitempty"`
	// Value is of any type, usually string or numeric.
	Value interface{} `xml:"value,omitempty" json:"value,omitempty"`
}

// String returns a BenchmarkValue's value nicely-formatted.
func (p BenchmarkValue) String() (r string) {
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
