// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResource_Processors(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		content  []byte
		expected []Processor
		wantErr  bool
	}{
		{
			name:   "Yaml pipeline",
			format: "yml",
			content: []byte(`---
description: Made up pipeline
processors:
# First processor.
- grok:
    tag: Extract header
    field: message
    patterns:
    - \[%{APACHE_TIME:apache.error.timestamp}\] \[%{LOGLEVEL:log.level}\]( \[client
      %{IPORHOST:source.address}(:%{POSINT:source.port})?\])? %{GREEDYDATA:message}
    - \[%{APACHE_TIME:apache.error.timestamp}\] \[%{DATA:apache.error.module}:%{LOGLEVEL:log.level}\]
      \[pid %{NUMBER:process.pid:long}(:tid %{NUMBER:process.thread.id:long})?\](
      \[client %{IPORHOST:source.address}(:%{POSINT:source.port})?\])? %{GREEDYDATA:message}
    pattern_definitions:
      APACHE_TIME: '%{DAY} %{MONTH} %{MONTHDAY} %{TIME} %{YEAR}'
    ignore_missing: true

- date:
    field: apache.error.timestamp
    target_field: '@timestamp'
    formats:
    - EEE MMM dd H:m:s yyyy
    - EEE MMM dd H:m:s.SSSSSS yyyy
    on_failure:
    - append:
        field: error.message
        value: '{{ _ingest.on_failure_message }}'
- set:
    description: Set event category
    field: event.category
    value: web
# Some script
- script:
    lang: painless
    source: >-
      [...]

- grok:
    field: source.address
    ignore_missing: true
    patterns:
    - ^(%{IP:source.ip}|%{HOSTNAME:source.domain})$
- rename:
    field: source.as.organization_name
    target_field: source.as.organization.name
    ignore_missing: true
on_failure:
- set:
    field: error.message
    value: '{{ _ingest.on_failure_message }}'
`),
			expected: []Processor{
				{Type: "grok", FirstLine: 5, LastLine: 16},
				{Type: "date", FirstLine: 18, LastLine: 27},
				{Type: "set", FirstLine: 28, LastLine: 31},
				{Type: "script", FirstLine: 33, LastLine: 35},
				{Type: "grok", FirstLine: 38, LastLine: 42},
				{Type: "rename", FirstLine: 43, LastLine: 46},
			},
		},
		{
			name:   "Json pipeline",
			format: "json",
			content: []byte(`{
  "description": "Pipeline for parsing silly logs.",
  "processors": [{"drop": {"if":"ctx.drop!=null"}},
	{
    "set": {
      "field": "event.ingested",
      "value": "{{_ingest.timestamp}}"
    }
  }, {"remove":{"field": "message"}}, {"set": {"field": "temp.duration","value":1234}},
  {
    "set":{
      "field": "event.kind",
      "value": "event"
    }
  }],
  "on_failure" : [{
    "set" : {
      "field" : "error.message",
      "value" : "{{ _ingest.on_failure_message }}"
    }
  }]
}
`),
			expected: []Processor{
				{Type: "drop", FirstLine: 3, LastLine: 3},
				{Type: "set", FirstLine: 4, LastLine: 7},
				{Type: "remove", FirstLine: 9, LastLine: 9},
				{Type: "set", FirstLine: 9, LastLine: 9},
				{Type: "set", FirstLine: 10, LastLine: 13},
			},
		},
		{
			name:     "Empty Yaml pipeline",
			format:   "yml",
			content:  []byte(``),
			expected: nil,
		},
		{
			name:     "Empty Json pipeline",
			format:   "json",
			content:  []byte(``),
			expected: nil,
		},
		{
			name:    "Json pipeline one liner",
			format:  "json",
			content: []byte(`{"processors": [{"drop": {}}]}`),
			expected: []Processor{
				{Type: "drop", FirstLine: 1, LastLine: 1},
			},
		},
		{
			name:    "Malformed Json pipeline",
			format:  "json",
			content: []byte(`{"processors": {"drop": {}}}`),
			wantErr: true,
		},
		{
			name:    "Broken Json",
			format:  "json",
			content: []byte(`{"processors": {"drop": {}},"`),
			wantErr: true,
		},
		{
			name:   "Malformed Yaml pipeline",
			format: "yml",
			content: []byte(`---
processors:
 foo:
  bar: baz`),
			wantErr: true,
		},
		{
			name:    "Malformed Yaml",
			format:  "yml",
			content: []byte(`foo123"`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Pipeline{
				Name:    tt.name,
				Format:  tt.format,
				Content: tt.content,
			}
			procs, err := p.Processors()
			if !tt.wantErr {
				if !assert.NoError(t, err) {
					t.Fatal(err)
				}
			} else {
				if !assert.Error(t, err) {
					t.Fatal("error expected")
				}
			}
			if !assert.Equal(t, tt.expected, procs) {
				t.Errorf("Processors() gotProcs = %v, want %v", procs, tt.expected)
			}
		})
	}
}
