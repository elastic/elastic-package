// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchrunner

import (
	"fmt"
)

type BenchmarkResult struct {
	// Type of benchmark
	Type string `json:"type"`
	// Package of the benchmark
	Package string `json:"package"`
	// DataStream of the benchmark
	DataStream string `json:"data_stream"`
	// Description of the benchmark run.
	Description string `json:"description,omitempty"`
	// Parameters used for this benchmark.
	Parameters []BenchmarkValue `json:"parameters,omitempty"`
	// Tests holds the results for the benchmark.
	Tests []BenchmarkTest `json:"test"`
}

// BenchmarkTest models a particular test performed during a benchmark.
type BenchmarkTest struct {
	// Name of this test.
	Name string `json:"name"`
	// Detailed benchmark tests will be printed to the output but not
	// included in file reports.
	Detailed bool `json:"-"`
	// Description of this test.
	Description string `json:"description,omitempty"`
	// Parameters for this test.
	Parameters []BenchmarkValue `json:"parameters,omitempty"`
	// Results of the test.
	Results []BenchmarkValue `json:"result"`
}

// BenchmarkValue represents a value (result or parameter)
// with an optional associated unit.
type BenchmarkValue struct {
	// Name of the value.
	Name string `json:"name"`
	// Description of the value.
	Description string `json:"description,omitempty"`
	// Unit used for this value.
	Unit string `json:"unit,omitempty"`
	// Value is of any type, usually string or numeric.
	Value interface{} `json:"value,omitempty"`
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
