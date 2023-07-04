// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package reporters

import (
	"fmt"
)

// Output represents an output for a benchmark report
type Output string

// OutputFunc defines the report writer function.
type OutputFunc func(Reportable) error

var reportOutputs = map[Output]OutputFunc{}

// RegisterOutput registers a benchmark report output.
func RegisterOutput(name Output, outputFunc OutputFunc) {
	reportOutputs[name] = outputFunc
}

// WriteReportable delegates writing of benchmark results to the registered benchmark report output
func WriteReportable(output Output, report Reportable) error {
	outputFunc, defined := reportOutputs[output]
	if !defined {
		return fmt.Errorf("unregistered benchmark report output: %s", output)
	}
	return outputFunc(report)
}
