// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	estest "github.com/elastic/elastic-package/internal/elasticsearch/test"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

func TestFindPolicyTemplateForInput(t *testing.T) {
	const policyTemplateName = "my_policy_template"
	const dataStreamName = "my_data_stream"
	const inputName = "logfile"

	var testCases = []struct {
		testName string
		err      string
		pkg      packages.PackageManifest
		input    string
	}{
		{
			testName: "single policy_template",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: nil,
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
				},
			},
			input: inputName,
		},
		{
			testName: "unspecified input name",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: nil,
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
				},
			},
		},
		{
			testName: "input matching",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: nil,
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
					{
						Name:        policyTemplateName + "1",
						DataStreams: nil,
						Inputs: []packages.Input{
							{
								Type: "not_" + inputName,
							},
						},
					},
				},
			},
			input: inputName,
		},
		{
			testName: "data stream not specified",
			err:      "no policy template was found",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: []string{"not_" + dataStreamName},
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
				},
			},
			input: inputName,
		},
		{
			testName: "multiple matches",
			err:      "ambiguous result",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: []string{dataStreamName},
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
					{
						Name:        policyTemplateName + "1",
						DataStreams: []string{dataStreamName},
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
				},
			},
			input: inputName,
		},
	}

	ds := &packages.DataStreamManifest{
		Name: dataStreamName,
		Streams: []packages.Stream{
			{Input: inputName},
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			name, err := findPolicyTemplateForInput(tc.pkg, ds, inputName)

			if tc.err != "" {
				require.Errorf(t, err, "expected err containing %q", tc.err)
				assert.Contains(t, err.Error(), tc.err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, policyTemplateName, name)
		})
	}
}

func TestCheckAgentLogs(t *testing.T) {
	var testCases = []struct {
		testName        string
		startingTime    string
		errorPatterns   []logsByContainer
		sampleLogs      map[string][]string
		expectedErrors  int
		expectedMessage []string
		expectedDetails []string
	}{
		{
			testName:     "all logs found",
			startingTime: "2023-05-15T12:00:00.000Z",
			errorPatterns: []logsByContainer{
				logsByContainer{
					containerName: "service",
					patterns: []logsRegexp{
						logsRegexp{
							includes: regexp.MustCompile(".*"),
						},
					},
				},
			},
			sampleLogs: map[string][]string{
				"service": []string{
					`service_1 | {"@timestamp": "2023-05-15T13:00:00.000Z", "message": "something"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:01.000Z", "message": "foo"}`,
				},
			},
			expectedErrors: 1,
			expectedMessage: []string{
				"test case failed: one or more errors found while examining service.log",
			},
			expectedDetails: []string{
				"[0] found error \"something\"\n[1] found error \"foo\"",
			},
		},
		{
			testName:     "remove old logs",
			startingTime: "2023-05-15T13:00:02.000Z",
			errorPatterns: []logsByContainer{
				logsByContainer{
					containerName: "service",
					patterns: []logsRegexp{
						logsRegexp{
							includes: regexp.MustCompile(".*"),
						},
					},
				},
			},
			sampleLogs: map[string][]string{
				"service": []string{
					`service_1 | {"@timestamp": "2023-05-15T13:00:00.000Z", "message": "something"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:05.000Z", "message": "foo"}`,
				},
			},
			expectedErrors:  1,
			expectedMessage: []string{"test case failed: one or more errors found while examining service.log"},
			expectedDetails: []string{"[0] found error \"foo\""},
		},
		{
			testName:     "all logs older",
			startingTime: "2023-05-15T14:00:00.000Z",
			errorPatterns: []logsByContainer{
				logsByContainer{
					containerName: "service",
					patterns: []logsRegexp{
						logsRegexp{
							includes: regexp.MustCompile(".*"),
						},
					},
				},
			},
			sampleLogs: map[string][]string{
				"service": []string{
					`service_1 | {"@timestamp": "2023-05-15T13:00:00.000Z", "message": "something"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:05.000Z", "message": "foo"}`,
				},
			},
			expectedErrors: 0,
		},
		{
			testName:     "filter logs by regex",
			startingTime: "2023-05-15T12:00:00.000Z",
			errorPatterns: []logsByContainer{
				logsByContainer{
					containerName: "service",
					patterns: []logsRegexp{
						logsRegexp{
							includes: regexp.MustCompile(".*thing$"),
						},
					},
				},
			},
			sampleLogs: map[string][]string{
				"service": []string{
					`service_1 | {"@timestamp": "2023-05-15T13:00:00.000Z", "message": "initial"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:02.000Z", "message": "something"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:05.000Z", "message": "foo"}`,
				},
			},
			expectedErrors:  1,
			expectedMessage: []string{"test case failed: one or more errors found while examining service.log"},
			expectedDetails: []string{"[0] found error \"something\""},
		},
		{
			testName:     "logs found for two services",
			startingTime: "2023-05-15T13:00:01.000Z",
			errorPatterns: []logsByContainer{
				logsByContainer{
					containerName: "service",
					patterns: []logsRegexp{
						logsRegexp{
							includes: regexp.MustCompile(".*thing$"),
						},
					},
				},
				logsByContainer{
					containerName: "external",
					patterns: []logsRegexp{
						logsRegexp{
							includes: regexp.MustCompile(" foo$"),
						},
					},
				},
			},
			sampleLogs: map[string][]string{
				"service": []string{
					`service_1 | {"@timestamp": "2023-05-15T13:00:00.000Z", "message": "service: initial"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:02.000Z", "message": "service: something"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:05.000Z", "message": "service: foo"}`,
				},
				"external": []string{
					`external_1 | {"@timestamp": "2023-05-15T13:00:00.000Z", "message": "external: initial"}`,
					`external_1 | {"@timestamp": "2023-05-15T13:00:05.000Z", "message": "external: foo"}`,
					`external_1 | {"@timestamp": "2023-05-15T13:00:08.000Z", "message": "external: any other foo"}`,
				},
			},
			expectedErrors: 2,
			expectedMessage: []string{
				"test case failed: one or more errors found while examining service.log",
				"test case failed: one or more errors found while examining external.log",
			},
			expectedDetails: []string{
				"[0] found error \"service: something\"",
				"[0] found error \"external: foo\"\n[1] found error \"external: any other foo\"",
			},
		},
		{
			testName:     "usage of includes and excludes",
			startingTime: "2023-05-15T12:00:00.000Z",
			errorPatterns: []logsByContainer{
				logsByContainer{
					containerName: "service",
					patterns: []logsRegexp{
						logsRegexp{
							includes: regexp.MustCompile("^(something|foo)"),
							excludes: []*regexp.Regexp{
								regexp.MustCompile("foo$"),
								regexp.MustCompile("42"),
							},
						},
					},
				},
			},
			sampleLogs: map[string][]string{
				"service": []string{
					`service_1 | {"@timestamp": "2023-05-15T13:00:00.000Z", "message": "something"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:05.000Z", "message": "something foo"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:10.000Z", "message": "foo bar"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:15.000Z", "message": "foo bar 42"}`,
					`service_1 | {"@timestamp": "2023-05-15T13:00:20.000Z", "message": "other message"}`,
				},
			},
			expectedErrors:  1,
			expectedMessage: []string{"test case failed: one or more errors found while examining service.log"},
			expectedDetails: []string{"[0] found error \"something\"\n[1] found error \"foo bar\""},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			logsDirTemp := t.TempDir()

			startTime, err := time.Parse(time.RFC3339, tc.startingTime)
			require.NoError(t, err)

			err = os.MkdirAll(filepath.Join(logsDirTemp, "logs"), 0755)
			require.NoError(t, err)

			var dump []stack.DumpResult
			for service, logs := range tc.sampleLogs {
				logsFile := filepath.Join(logsDirTemp, "logs", fmt.Sprintf("%s.log", service))
				file, err := os.Create(logsFile)
				require.NoError(t, err)

				_, err = file.WriteString(strings.Join(logs, "\n"))
				require.NoError(t, err)
				file.Close()

				dump = append(dump, stack.DumpResult{
					ServiceName: service,
					LogsFile:    logsFile,
				})
			}

			tester := tester{
				testFolder: testrunner.TestFolder{
					Package:    "package",
					DataStream: "datastream",
				},
			}
			results, err := tester.checkAgentLogs(dump, startTime, tc.errorPatterns)
			require.NoError(t, err)
			require.Len(t, results, tc.expectedErrors)

			if tc.expectedErrors == 0 {
				assert.Nil(t, results)
				return
			}

			for i := 0; i < tc.expectedErrors; i++ {
				assert.Equal(t, tc.expectedMessage[i], results[i].FailureMsg)
				assert.Equal(t, tc.expectedDetails[i], results[i].FailureDetails)
			}
		})
	}
}

func TestIsSyntheticSourceModeEnabled(t *testing.T) {
	cases := []struct {
		title          string
		record         string
		dataStreamName string
		expected       bool
	}{
		{
			title:          "no synthetics",
			record:         "testdata/elasticsearch-8-mock-synthetic-mode-nginx",
			dataStreamName: "logs-nginx.access-12345",
			expected:       false,
		},
		{
			// This test case is generated with -U stack.logsdb_enabled=true
			title:          "logsdb mode but no synthetics otherwise",
			record:         "testdata/elasticsearch-8-mock-synthetic-mode-nginx-logsdb",
			dataStreamName: "logs-nginx.access-12345",
			expected:       true,
		},
		{
			title:          "time_series mode",
			record:         "testdata/elasticsearch-8-mock-synthetic-mode-couchdb",
			dataStreamName: "metrics-couchdb.server-12345",
			expected:       true,
		},
		{
			// This test case is generated with the logs_synthetic_mode test package from the Package Spec.
			title:          "synthetic mode explicitly enabled in package",
			record:         "testdata/elasticsearch-8-mock-synthetic-mode-dummy",
			dataStreamName: "logs-logs_synthetic_mode.synthetic-12345",
			expected:       true,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			client := estest.NewClient(t, c.record, nil)
			enabled, err := isSyntheticSourceModeEnabled(t.Context(), client.API, c.dataStreamName)
			require.NoError(t, err)
			assert.Equal(t, c.expected, enabled)
		})
	}
}
