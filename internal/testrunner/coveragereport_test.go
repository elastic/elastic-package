// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCoverageReport(t *testing.T) {
	workDir, err := os.Getwd()
	require.NoError(t, err)
	packageRootPath := filepath.Join(workDir, "my", "path", "package")
	tests := []struct {
		name           string
		rootPath       string
		packageName    string
		packageType    string
		coverageFormat string
		timestamp      int64
		testType       TestType
		results        []TestResult
		expected       CoverageReport
	}{
		{
			name:           "generate custom cobertura coverage",
			testType:       "system",
			rootPath:       packageRootPath,
			packageName:    "mypackage",
			packageType:    "integration",
			coverageFormat: "cobertura",
			timestamp:      10,
			results: []TestResult{
				{
					Name:        "test1",
					Package:     "mypackage",
					DataStream:  "metrics",
					TimeElapsed: 1 * time.Second,
					Coverage:    nil,
				},
				{
					Name:        "test2",
					Package:     "mypackage",
					DataStream:  "logs",
					TimeElapsed: 2 * time.Second,
					Coverage:    nil,
				},
			},
			expected: &CoberturaCoverage{
				Version:   "",
				Timestamp: 10,
				Packages: []*CoberturaPackage{
					{
						Name: "mypackage",
						Classes: []*CoberturaClass{
							{
								Name:     "system",
								Filename: filepath.Join("internal", "testrunner", "my", "path", "mypackage", "data_stream", "logs", "manifest.yml"),
								Methods: []*CoberturaMethod{
									{
										Name:      "OK",
										Signature: "",
										Lines: []*CoberturaLine{
											{
												Number: 3,
												Hits:   1,
											},
										},
									},
								},
								Lines: []*CoberturaLine{
									{
										Number: 3,
										Hits:   1,
									},
								},
							},
							{
								Name:     "system",
								Filename: filepath.Join("internal", "testrunner", "my", "path", "mypackage", "data_stream", "metrics", "manifest.yml"),
								Methods: []*CoberturaMethod{
									{
										Name:      "OK",
										Signature: "",
										Lines: []*CoberturaLine{
											{
												Number: 3,
												Hits:   1,
											},
										},
									},
								},
								Lines: []*CoberturaLine{
									{
										Number: 3,
										Hits:   1,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:           "generate custom generic coverage",
			testType:       "system",
			rootPath:       packageRootPath,
			packageName:    "mypackage",
			packageType:    "integration",
			coverageFormat: "generic",
			timestamp:      10,
			results: []TestResult{
				{
					Name:        "test1",
					Package:     "mypackage",
					DataStream:  "metrics",
					TimeElapsed: 1 * time.Second,
					Coverage:    nil,
				},
				{
					Name:        "test2",
					Package:     "mypackage",
					DataStream:  "logs",
					TimeElapsed: 2 * time.Second,
					Coverage:    nil,
				},
			},
			expected: &GenericCoverage{
				Version: 1,
				Files: []*GenericFile{
					{
						Path: filepath.Join("internal", "testrunner", "my", "path", "mypackage", "data_stream", "logs", "manifest.yml"),
						Lines: []*GenericLine{
							{
								LineNumber: 3,
								Covered:    true,
							},
						},
					},
					{
						Path: filepath.Join("internal", "testrunner", "my", "path", "mypackage", "data_stream", "metrics", "manifest.yml"),
						Lines: []*GenericLine{
							{
								LineNumber: 3,
								Covered:    true,
							},
						},
					},
				},
				TestType:  "Coverage for system test",
				Timestamp: 10,
			},
		},
		{
			name:           "use provided generic coverage",
			testType:       "system",
			rootPath:       packageRootPath,
			packageName:    "mypackage",
			packageType:    "integration",
			coverageFormat: "generic",
			timestamp:      10,
			results: []TestResult{
				{
					Name:        "test1",
					Package:     "mypackage",
					DataStream:  "metrics",
					TimeElapsed: 1 * time.Second,
					Coverage: &GenericCoverage{
						Version: 1,
						Files: []*GenericFile{
							{
								Path: filepath.Join("internal", "testrunner", "my", "path", "mypackage", "data_stream", "metrics", "foo.yml"),
								Lines: []*GenericLine{
									{
										LineNumber: 1,
										Covered:    true,
									},
									{
										LineNumber: 2,
										Covered:    true,
									},
								},
							},
						},
						TestType:  "Coverage for system test",
						Timestamp: 20,
					},
				},
			},
			expected: &GenericCoverage{
				Version: 1,
				Files: []*GenericFile{
					{
						Path: filepath.Join("internal", "testrunner", "my", "path", "mypackage", "data_stream", "metrics", "foo.yml"),
						Lines: []*GenericLine{
							{
								LineNumber: 1,
								Covered:    true,
							},
							{
								LineNumber: 2,
								Covered:    true,
							},
						},
					},
				},
				TestType:  "Coverage for system test",
				Timestamp: 20,
			},
		},
		{
			name:           "generic coverage for an input package",
			testType:       "asset",
			rootPath:       packageRootPath,
			packageName:    "mypackage",
			packageType:    "input",
			coverageFormat: "generic",
			timestamp:      10,
			results: []TestResult{
				{
					Name:        "test1",
					Package:     "mypackage",
					DataStream:  "",
					TimeElapsed: 1 * time.Second,
					Coverage:    nil,
				},
			},
			expected: &GenericCoverage{
				Version: 1,
				Files: []*GenericFile{
					{
						Path: filepath.Join("internal", "testrunner", "my", "path", "mypackage", "manifest.yml"),
						Lines: []*GenericLine{
							{
								LineNumber: 1,
								Covered:    true,
							},
						},
					},
				},
				TestType:  "Coverage for asset test",
				Timestamp: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := createCoverageReport(tt.rootPath, tt.packageName, tt.packageType, tt.testType, tt.results, tt.coverageFormat, tt.timestamp)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, report)
		})
	}
}
