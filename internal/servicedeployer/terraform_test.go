// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddTerraformOutputs(t *testing.T) {
	var testCases = []struct {
		testName      string
		err           string
		svcInfo       ServiceInfo
		runId         string
		content       []byte
		expectedProps map[string]interface{}
		expectedError bool
	}{
		{
			testName: "invalid_json_output",
			runId:    "987987",
			svcInfo: ServiceInfo{
				Test: struct{ RunID string }{"987987"},
			},
			content: []byte(
				``,
			),
			expectedProps: nil,
			expectedError: true,
		},
		{
			testName: "empty_json_output",
			runId:    "v",
			svcInfo: ServiceInfo{
				Test: struct{ RunID string }{"9887"},
			},
			content: []byte(
				`{}`,
			),
			expectedProps: nil,
		},
		{
			testName: "single_value_output",
			runId:    "99999",
			svcInfo: ServiceInfo{
				Test: struct{ RunID string }{"99999"},
			},
			content: []byte(
				`{
				"queue_url": {
				  "sensitive": false,
				  "type": "string",
				  "value": "https://sqs.us-east-1.amazonaws.com/1234654/elastic-package-aws-logs-queue-someId"
				}
			   }`,
			),
			expectedProps: map[string]interface{}{
				"TF_OUTPUT_queue_url": "https://sqs.us-east-1.amazonaws.com/1234654/elastic-package-aws-logs-queue-someId",
			},
		},
		{
			testName: "add_single_value_output",
			runId:    "99999",
			svcInfo: ServiceInfo{
				Test: struct{ RunID string }{"99999"},
				CustomProperties: map[string]interface{}{
					"TF_foo": "bar",
				},
			},
			content: []byte(
				`{
				"queue_url": {
				  "sensitive": false,
				  "type": "string",
				  "value": "https://sqs.us-east-1.amazonaws.com/1234654/elastic-package-aws-logs-queue-someId"
				}
			   }`,
			),
			expectedProps: map[string]interface{}{
				"TF_OUTPUT_queue_url": "https://sqs.us-east-1.amazonaws.com/1234654/elastic-package-aws-logs-queue-someId",
				"TF_foo":              "bar",
			},
		},
		{
			testName: "multiple_value_output",
			runId:    "23465",
			svcInfo: ServiceInfo{
				Test: struct{ RunID string }{"23465"},
			},
			content: []byte(
				`{
				"queue_url": {
				  "sensitive": false,
				  "type": "string",
				  "value": "https://sqs.us-east-1.amazonaws.com/1234654/elastic-package-aws-logs-queue-someId"
				},
				"instance_id": {
					"sensitive": false,
					"type": "string",
					"value": "some-random-id"
				  }
			   }`,
			),
			expectedProps: map[string]interface{}{
				"TF_OUTPUT_queue_url":   "https://sqs.us-east-1.amazonaws.com/1234654/elastic-package-aws-logs-queue-someId",
				"TF_OUTPUT_instance_id": "some-random-id",
			},
		},
		{
			testName: "complex_value_output",
			runId:    "078907890",
			svcInfo: ServiceInfo{
				Test: struct{ RunID string }{"078907890"},
			},
			content: []byte(
				`{
					"queue_url": {
					  "sensitive": false,
					  "type": "string",
					  "value": "https://sqs.us-east-1.amazonaws.com/1234654/elastic-package-aws-logs-queue-someId"
					},
					"triangle_output": {
					  "sensitive": false,
					  "type": [
						"object",
						{
						  "description": "string",
						  "s_one": "number",
						  "s_three": "number",
						  "s_two": "number"
						}
					  ],
					  "value": {
						"value": "this is a triangle",
						"s_one": 1,
						"s_three": 2.5,
						"s_two": 2.5
					  }
					}
				}`,
			),
			expectedProps: map[string]interface{}{
				"TF_OUTPUT_queue_url": "https://sqs.us-east-1.amazonaws.com/1234654/elastic-package-aws-logs-queue-someId",
				"TF_OUTPUT_triangle_output": map[string]any{
					"s_one":   1.0,
					"s_three": 2.5,
					"s_two":   2.5,
					"value":   "this is a triangle",
				},
			},
		},
	}

	t.Parallel()
	for _, tc := range testCases {

		t.Run(tc.testName, func(t *testing.T) {
			tc.svcInfo.OutputDir = t.TempDir()

			if err := os.WriteFile(tc.svcInfo.OutputDir+"/tfOutputValues.json", tc.content, 0777); err != nil {
				t.Fatal(err)
			}

			// Test that the terraform output values are generated correctly
			err := addTerraformOutputs(&tc.svcInfo)
			if tc.expectedError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedProps, tc.svcInfo.CustomProperties)
		})
	}
}
