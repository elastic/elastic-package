// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPipelineStats(t *testing.T) {
	for _, testcase := range []struct {
		title     string
		pipelines []Pipeline
		body      string
		expected  PipelineStatsMap
		isErr     bool
	}{
		{
			title: "empty",
			body: `
				{
					"nodes": {
						"node1": {}
					}
				}`,
			expected: PipelineStatsMap{},
		},
		{
			title: "bad format",
			body:  `{}`,
			isErr: true,
		},
		{
			title: "bad JSON",
			body:  `?`,
			isErr: true,
		},
		{
			title: "multiple nodes",
			body: `
				{
					"nodes": {
						"node1":{},
						"node2":{}
					}
				}`,
			isErr: true,
		},
		{
			title: "missing pipelines",
			body: `
				{
					"nodes": {
						"node1": {}
					}
				}`,
			pipelines: []Pipeline{
				{Name: "p"},
			},
			isErr: true,
		},
		{
			title: "bad processor",
			body: `
				{
					"nodes": {
						"node1": {
							"ingest": {
								"pipelines": {
									"p1": {
										"processors": [{}]
									}
								}
							}
						}
					}
				}`,
			pipelines: []Pipeline{
				{Name: "p1"},
			},
			isErr: true,
		},
		{
			title: "valid result",
			pipelines: []Pipeline{
				{Name: "p2"},
			},
			body: `
				{
					"nodes": {
						"node1": {
							"ingest": {
								"pipelines": {
									"p1": {
										"count": 1,
										"current": 2,
										"failed": 3,
										"time_in_millis": 4,
										"processors": [
											{
												"compound:CompoundProcessor-null": {
													"type": "compound",
													"stats": {
														"count": 5,
														"current": 6,
														"failed": 7,
														"time_in_millis": 8
													}
												}
											}
										]
									},
									"p2": {
										"count": 9,
										"current": 10,
										"failed": 11,
										"time_in_millis": 12,
										"processors": [
											{
												"append": {
													"type": "conditional",
													"stats": {
														"count": 13,
														"current": 14,
														"failed": 15,
														"time_in_millis": 16
													}
												}
											},
											{
												"geoip": {
													"type": "geoip",
													"stats": {
														"count": 17,
														"current": 18,
														"failed": 19,
														"time_in_millis": 20
													}
												}
											}
										]
									}
								}
							}
						}
					}
				}`,
			expected: PipelineStatsMap{
				"p2": PipelineStats{
					StatsRecord: StatsRecord{
						Count:        9,
						Current:      10,
						Failed:       11,
						TimeInMillis: 12,
					},
					Processors: []ProcessorStats{
						{
							Type:        "append",
							Conditional: true,
							Stats: StatsRecord{
								Count:        13,
								Current:      14,
								Failed:       15,
								TimeInMillis: 16,
							},
						},
						{
							Type: "geoip",
							Stats: StatsRecord{
								Count:        17,
								Current:      18,
								Failed:       19,
								TimeInMillis: 20,
							},
						},
					},
				},
			},
		},
	} {
		t.Run(testcase.title, func(t *testing.T) {
			stats, err := getPipelineStats([]byte(testcase.body), testcase.pipelines)
			if testcase.isErr {
				if !assert.Error(t, err) {
					t.Fatal("error expected")
				}
				return
			}
			if !assert.NoError(t, err) {
				t.Fatal(err)
			}
			assert.Equal(t, testcase.expected, stats)
		})
	}
}
