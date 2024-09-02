// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/testrunner"
)

func TestGenericCoverageForSinglePipeline(t *testing.T) {
	for _, testcase := range []struct {
		title                string
		pipelineRelPath      string
		src                  []ingest.Processor
		pstats               ingest.PipelineStats
		expectedLinesCovered int64
		expectedFile         *testrunner.GenericFile
	}{
		{
			title:           "Single Processor - covered",
			pipelineRelPath: "",
			src: []ingest.Processor{
				{Type: "append", FirstLine: 1, LastLine: 1},
			},
			pstats: ingest.PipelineStats{
				StatsRecord: ingest.StatsRecord{
					Count:        9,
					Current:      10,
					Failed:       11,
					TimeInMillis: 12,
				},
				Processors: []ingest.ProcessorStats{
					{
						Type:        "append",
						Conditional: true,
						Stats: ingest.StatsRecord{
							Count:        13,
							Current:      14,
							Failed:       15,
							TimeInMillis: 16,
						},
					},
				},
			},
			expectedLinesCovered: 1,
			expectedFile: &testrunner.GenericFile{
				Path: "",
				Lines: []*testrunner.GenericLine{
					{LineNumber: 1, Covered: true},
				},
			},
		},
		{
			title:           "Single Processor - not covered",
			pipelineRelPath: "",
			src: []ingest.Processor{
				{Type: "append", FirstLine: 1, LastLine: 1},
			},
			pstats: ingest.PipelineStats{
				StatsRecord: ingest.StatsRecord{
					Count:        9,
					Current:      10,
					Failed:       11,
					TimeInMillis: 12,
				},
				Processors: []ingest.ProcessorStats{
					{
						Type:        "append",
						Conditional: true,
						Stats: ingest.StatsRecord{
							Count:        0,
							Current:      14,
							Failed:       15,
							TimeInMillis: 16,
						},
					},
				},
			},
			expectedLinesCovered: 0,
			expectedFile: &testrunner.GenericFile{
				Path: "",
				Lines: []*testrunner.GenericLine{
					{LineNumber: 1, Covered: false},
				},
			},
		},
		{
			title:           "Multi Processor - covered",
			pipelineRelPath: "",
			src: []ingest.Processor{
				{Type: "append", FirstLine: 1, LastLine: 1},
				{Type: "geoip", FirstLine: 2, LastLine: 2},
			},
			pstats: ingest.PipelineStats{
				StatsRecord: ingest.StatsRecord{
					Count:        9,
					Current:      10,
					Failed:       11,
					TimeInMillis: 12,
				},
				Processors: []ingest.ProcessorStats{
					{
						Type:        "append",
						Conditional: true,
						Stats: ingest.StatsRecord{
							Count:        13,
							Current:      14,
							Failed:       15,
							TimeInMillis: 16,
						},
					},
					{
						Type: "geoip",
						Stats: ingest.StatsRecord{
							Count:        17,
							Current:      18,
							Failed:       19,
							TimeInMillis: 20,
						},
					},
				},
			},
			expectedLinesCovered: 2,
			expectedFile: &testrunner.GenericFile{
				Path: "",
				Lines: []*testrunner.GenericLine{
					{LineNumber: 1, Covered: true},
					{LineNumber: 2, Covered: true},
				},
			},
		},
	} {
		t.Run(testcase.title, func(t *testing.T) {
			linesCoveredResult, fileResult, _ := genericCoverageForSinglePipeline(testcase.pipelineRelPath, testcase.src, testcase.pstats)
			assert.Equal(t, testcase.expectedLinesCovered, linesCoveredResult)
			assert.Equal(t, testcase.expectedFile, fileResult)
		})
	}
}
