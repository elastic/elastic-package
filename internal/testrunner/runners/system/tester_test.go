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

	"github.com/elastic/elastic-package/internal/common"
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
		pkg      *packages.PackageManifest
		input    string
	}{
		{
			testName: "single policy_template",
			pkg: &packages.PackageManifest{
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
			pkg: &packages.PackageManifest{
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
			pkg: &packages.PackageManifest{
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
			pkg: &packages.PackageManifest{
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
			pkg: &packages.PackageManifest{
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
			name, err := packages.FindPolicyTemplateForInput(tc.pkg, ds, inputName)

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
func TestSearchDataStreams(t *testing.T) {
	const pattern = "*-foo.bar-default"

	t.Run("happy path returns two streams", func(t *testing.T) {
		client := estest.NewClient(t, "testdata/elasticsearch-8-mock-discover-datastreams-found", nil)
		r := &tester{esAPI: client.API}

		streams, err := r.searchDataStreams(t.Context(), []string{pattern})
		require.NoError(t, err)
		require.Len(t, streams, 2)

		assert.Equal(t, "logs-foo.bar-default", streams[0].name)
		assert.Equal(t, "logs-foo.bar", streams[0].indexTemplate)

		assert.Equal(t, "metrics-foo.bar-default", streams[1].name)
		assert.Equal(t, "metrics-foo.bar", streams[1].indexTemplate)
	})

	t.Run("404 returns empty slice with no error", func(t *testing.T) {
		client := estest.NewClient(t, "testdata/elasticsearch-8-mock-discover-datastreams-notfound", nil)
		r := &tester{esAPI: client.API}

		streams, err := r.searchDataStreams(t.Context(), []string{pattern})
		require.NoError(t, err)
		assert.Empty(t, streams)
	})
}

func TestDiscoverDataStreams(t *testing.T) {
	const pattern = "*-myreceiver.otel-default"

	t.Run("returns error when no streams appear within timeout", func(t *testing.T) {
		client := estest.NewClient(t, "testdata/elasticsearch-8-mock-wait-for-all-datastreams-timeout", nil)
		r := &tester{esAPI: client.API}
		cfg := &testConfig{
			WaitForDataTimeout:          100 * time.Millisecond,
			WaitForDynamicStreamsStable: 2 * time.Second,
		}

		_, err := r.discoverDataStreams(t.Context(), cfg, []string{pattern})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no data streams matching")
	})

	t.Run("phase 2 picks up late-arriving stream", func(t *testing.T) {
		client := estest.NewClient(t, "testdata/elasticsearch-8-mock-wait-for-all-datastreams", nil)
		r := &tester{esAPI: client.API}
		cfg := &testConfig{
			WaitForDynamicStreamsStable: 2 * time.Second,
		}

		streams, err := r.discoverDataStreams(t.Context(), cfg, []string{pattern})
		require.NoError(t, err)
		require.Len(t, streams, 2)

		names := make(map[string]string)
		for _, s := range streams {
			names[s.name] = s.indexTemplate
		}
		assert.Equal(t, "logs-myreceiver.otel", names["logs-myreceiver.otel-default"])
		assert.Equal(t, "metrics-myreceiver.otel", names["metrics-myreceiver.otel-default"])
	})
}

func TestBuildDataStreamName(t *testing.T) {
	cases := []struct {
		title          string
		dsType         string
		dsDataset      string
		namespace      string
		policyTemplate packages.PolicyTemplate
		packageType    string
		expected       string
	}{
		{
			title:          "non-otelcol input: no suffix added",
			dsType:         "logs",
			dsDataset:      "nginx.access",
			namespace:      "default",
			policyTemplate: packages.PolicyTemplate{Input: "logfile"},
			packageType:    "integration",
			expected:       "logs-nginx.access-default",
		},
		{
			title:          "otelcol input: .otel suffix appended",
			dsType:         "logs",
			dsDataset:      "httpcheck",
			namespace:      "default",
			policyTemplate: packages.PolicyTemplate{Input: otelCollectorInputName},
			packageType:    "input",
			expected:       "logs-httpcheck.otel-default",
		},
		{
			title:          "otelcol input: no double-suffix when dataset already ends in .otel",
			dsType:         "logs",
			dsDataset:      "generic.otel",
			namespace:      "default",
			policyTemplate: packages.PolicyTemplate{Input: otelCollectorInputName},
			packageType:    "input",
			expected:       "logs-generic.otel-default",
		},
		{
			title:          "otelcol input on integration package type: no suffix added",
			dsType:         "metrics",
			dsDataset:      "myreceiver",
			namespace:      "default",
			policyTemplate: packages.PolicyTemplate{Input: otelCollectorInputName},
			packageType:    "integration",
			expected:       "metrics-myreceiver-default",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			got := BuildDataStreamName(c.dsType, c.dsDataset, c.namespace, c.policyTemplate, c.packageType)
			assert.Equal(t, c.expected, got)
		})
	}
}

func TestExpectedDatasets(t *testing.T) {
	cases := []struct {
		title    string
		scenario *scenarioTest
		expected []string
	}{
		{
			title: "non-otelcol package: dataset returned as-is",
			scenario: &scenarioTest{
				dataStreamDataset: "nginx.access",
				policyTemplate:    packages.PolicyTemplate{Input: "logfile"},
			},
			expected: []string{"nginx.access"},
		},
		{
			title: "otelcol input: .otel suffix appended and generic.otel included",
			scenario: &scenarioTest{
				dataStreamDataset: "httpcheck",
				policyTemplate:    packages.PolicyTemplate{Input: otelCollectorInputName},
			},
			expected: []string{"httpcheck.otel", "generic.otel"},
		},
		{
			title: "otelcol input with generic.otel dataset: no double-suffix",
			scenario: &scenarioTest{
				dataStreamDataset: "generic.otel",
				policyTemplate:    packages.PolicyTemplate{Input: otelCollectorInputName},
			},
			expected: []string{"generic.otel", "generic.otel"},
		},
		{
			title: "otelcol dynamic_signal_types: uses stored dataset, not policyTemplate.Name",
			scenario: &scenarioTest{
				dataStreamDataset: "sqlserverreceiver",
				policyTemplate: packages.PolicyTemplate{
					Name:               "sqlserverreceiver",
					Input:              otelCollectorInputName,
					DynamicSignalTypes: true,
				},
			},
			expected: []string{"sqlserverreceiver.otel", "generic.otel"},
		},
		{
			title: "otelcol dynamic_signal_types with generic.otel dataset: no double-suffix",
			scenario: &scenarioTest{
				dataStreamDataset: "generic.otel",
				policyTemplate: packages.PolicyTemplate{
					Name:               "sqlserverreceiver",
					Input:              otelCollectorInputName,
					DynamicSignalTypes: true,
				},
			},
			expected: []string{"generic.otel", "generic.otel"},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			r := &tester{}
			got, err := r.expectedDatasets(c.scenario, nil)
			require.NoError(t, err)
			assert.Equal(t, c.expected, got)
		})
	}
}

func TestBuildDataStreamScenarios(t *testing.T) {
	t.Run("standard single stream", func(t *testing.T) {
		r := &tester{pkgManifest: &packages.PackageManifest{Type: "integration"}}
		pt := packages.PolicyTemplate{Name: "bar"}
		cfg := &testConfig{}

		got, err := r.buildDataStreamScenarios(t.Context(), "logs", "foo.bar", "default", pt, cfg)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "logs-foo.bar-default", got[0].dataStream)
		assert.Equal(t, "logs-foo.bar", got[0].indexTemplateName)
	})

	t.Run("explicit signal_types produce one entry per type", func(t *testing.T) {
		client := estest.NewClient(t, "testdata/elasticsearch-8-mock-build-datastream-scenarios-explicit-signal-types", nil)
		r := &tester{pkgManifest: &packages.PackageManifest{Type: "input"}, esAPI: client.API}
		pt := packages.PolicyTemplate{Name: "myreceiver", Input: "otelcol", DynamicSignalTypes: true}
		cfg := &testConfig{
			SignalTypes:                 []string{"logs", "metrics"},
			WaitForDynamicStreamsStable: 2 * time.Second,
		}

		got, err := r.buildDataStreamScenarios(t.Context(), "logs", "myreceiver", "default", pt, cfg)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "logs-myreceiver.otel-default", got[0].dataStream)
		assert.Equal(t, "logs-myreceiver", got[0].indexTemplateName)
		assert.Equal(t, "metrics-myreceiver.otel-default", got[1].dataStream)
		assert.Equal(t, "metrics-myreceiver", got[1].indexTemplateName)
	})
}

func TestPipelineErrorMessage(t *testing.T) {
	testCases := []struct {
		name     string
		doc      common.MapStr
		expected string
	}{
		{
			name:     "empty doc",
			doc:      common.MapStr{},
			expected: "",
		},
		{
			name: "doc without event.kind",
			doc: common.MapStr{
				"message": "something",
			},
			expected: "",
		},
		{
			name: "event.kind is not pipeline_error",
			doc: common.MapStr{
				"event": common.MapStr{
					"kind": "event",
				},
			},
			expected: "",
		},
		{
			name: "event.kind is non-string",
			doc: common.MapStr{
				"event": common.MapStr{
					"kind": 42,
				},
			},
			expected: "",
		},
		{
			name: "pipeline_error without error.message",
			doc: common.MapStr{
				"event": common.MapStr{
					"kind": "pipeline_error",
				},
			},
			expected: "found pipeline_error in document: no error message",
		},
		{
			name: "pipeline_error with empty error.message",
			doc: common.MapStr{
				"event": common.MapStr{
					"kind": "pipeline_error",
				},
				"error": common.MapStr{
					"message": "",
				},
			},
			expected: "found pipeline_error in document with error message: \"\"",
		},
		{
			name: "pipeline_error with non-string error.message",
			doc: common.MapStr{
				"event": common.MapStr{
					"kind": "pipeline_error",
				},
				"error": common.MapStr{
					"message": 123,
				},
			},
			expected: "found pipeline_error in document: no error message",
		},
		{
			name: "pipeline_error with error.message",
			doc: common.MapStr{
				"event": common.MapStr{
					"kind": "pipeline_error",
				},
				"error": common.MapStr{
					"message": "ingest pipeline failed",
				},
			},
			expected: "found pipeline_error in document with error message: \"ingest pipeline failed\"",
		},
		{
			name: "pipeline_error with error.message as array",
			doc: common.MapStr{
				"event": common.MapStr{
					"kind": "pipeline_error",
				},
				"error": common.MapStr{
					"message": []any{"ingest pipeline failed"},
				},
			},
			expected: "found pipeline_error in document with error message: \"ingest pipeline failed\"",
		},
		{
			name: "pipeline_error using synthetic source mode",
			doc: common.MapStr{
				"event": common.MapStr{
					"kind": []any{"pipeline_error"},
				},
				"error": common.MapStr{
					"message": []any{"ingest pipeline failed"},
				},
			},
			expected: "found pipeline_error in document with error message: \"ingest pipeline failed\"",
		},
		{
			name: "unexpected type for event field",
			doc: common.MapStr{
				"event": []any{"foo"},
				"error": common.MapStr{
					"message": "ingest pipeline failed",
				},
			},
			expected: "",
		},
		{
			name: "unexpected type for error field",
			doc: common.MapStr{
				"event": common.MapStr{
					"kind": "pipeline_error",
				},
				"error": []any{
					"404 error code",
				},
			},
			expected: "found pipeline_error in document: no error message",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := pipelineErrorMessage(tc.doc)
			assert.Equal(t, tc.expected, got)
		})
	}
}
