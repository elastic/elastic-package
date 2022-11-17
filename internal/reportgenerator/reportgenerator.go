// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package reportgenerator

import (
	"fmt"
)

type ReportOptions struct {
	NewPath   string
	OldPath   string
	Threshold float64
	Full      bool
}

// ReportType represents the various supported report generators
type ReportType string

// ReportGenerator is the interface all report generators must implement.
type ReportGenerator interface {
	// Type returns the report generator's type.
	Type() ReportType

	// String returns the human-friendly name of the report generator.
	String() string

	// Format returns the format used by the report.
	Format() string

	// Run executes the benchmark runner.
	Generate(ReportOptions) ([]byte, error)
}

var generators = map[ReportType]ReportGenerator{}

// RegisterGenerator method registers a report generator.
func RegisterGenerator(g ReportGenerator) {
	generators[g.Type()] = g
}

// Generate method delegates execution to the registered report generator, based on the report type.
func Generate(ReportType ReportType, options ReportOptions) ([]byte, error) {
	gen, defined := generators[ReportType]
	if !defined {
		return nil, fmt.Errorf("unregistered report generator: %s", ReportType)
	}
	return gen.Generate(options)
}

// ReportGenerators returns registered report generators.
func ReportGenerators() map[ReportType]ReportGenerator {
	return generators
}
