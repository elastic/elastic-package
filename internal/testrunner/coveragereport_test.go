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
		testType       TestType
		results        []TestResult
		expected       CoverageReport
	}{
		{
			name:           "use provided generic coverage",
			testType:       "system",
			rootPath:       packageRootPath,
			packageName:    "mypackage",
			packageType:    "integration",
			coverageFormat: "generic",
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
						TestType: "Coverage for system test",
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
				TestType: "Coverage for system test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := createCoverageReport(tt.rootPath, workDir, tt.packageName, tt.packageType, tt.testType, tt.results, tt.coverageFormat)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, report)
		})
	}
}
