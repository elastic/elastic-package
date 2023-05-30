// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindPolicyTemplateForInput(t *testing.T) {

	var testCases = []struct {
		testName      string
		err           string
		ctxt          ServiceContext
		runId         string
		content       []byte
		expectedProps map[string]interface{}
	}{
		{
			testName: "single_value_output",
			runId:    "99999",
			ctxt: ServiceContext{
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
			testName: "multiple_value_output",
			runId:    "23465",
			ctxt: ServiceContext{
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
			ctxt: ServiceContext{
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
						"description": "this is a triangle",
						"s_one": 1,
						"s_three": 2.5,
						"s_two": 2.5
					  }
					}
				}`,
			),
			expectedProps: map[string]interface{}{
				"TF_OUTPUT_queue_url":                   "https://sqs.us-east-1.amazonaws.com/1234654/elastic-package-aws-logs-queue-someId",
				"TF_OUTPUT_triangle_output.s_one":       1.0,
				"TF_OUTPUT_triangle_output.s_two":       2.5,
				"TF_OUTPUT_triangle_output.s_three":     2.5,
				"TF_OUTPUT_triangle_output.description": "this is a triangle",
			},
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			tc.ctxt.CustomProperties = make(map[string]interface{})

			os.Mkdir("/tmp/"+tc.runId, os.ModePerm)
			file, _ := os.OpenFile("/tmp/"+tc.runId+"/tfOutputValues.json", os.O_CREATE|os.O_WRONLY, 0777)
			file.Write(tc.content)

			defer func() {
				_ = file.Close()
				_ = os.RemoveAll("/tmp/" + tc.runId + "/")
			}()

			// Test that the terraform output values are generated correctly
			addTerraformOutputs(tc.ctxt)
			assert.Equal(t, tc.expectedProps, tc.ctxt.CustomProperties)
		})
	}
}
