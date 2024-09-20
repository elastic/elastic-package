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
    - grok:
        tag: Extract header
        field: message
        patterns:
        - \[%{APACHE_TIME:apache.error.timestamp}\] \[%{LOGLEVEL:log.level}\]( \[client%{IPORHOST:source.address}(:%{POSINT:source.port})?\])? %{GREEDYDATA:message}
        - \[%{APACHE_TIME:apache.error.timestamp}\] \[%{DATA:apache.error.module}:%{LOGLEVEL:log.level}\]
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
				{Type: "grok", FirstLine: 4, LastLine: 11},
				{Type: "date", FirstLine: 12, LastLine: 21},
				{Type: "set", FirstLine: 22, LastLine: 26},
				{Type: "script", FirstLine: 27, LastLine: 29},
				{Type: "grok", FirstLine: 30, LastLine: 34},
				{Type: "rename", FirstLine: 35, LastLine: 38},
			},
		},
		{
			name:   "Yaml pipeline",
			format: "yml",
			content: []byte(`---
description: Made up pipeline
processors:
    - grok:
        tag: Extract header
        field: message
        patterns:
        - \[%{APACHE_TIME:apache.error.timestamp}\] \[%{LOGLEVEL:log.level}\]( \[client%{IPORHOST:source.address}(:%{POSINT:source.port})?\])? %{GREEDYDATA:message}
        - \[%{APACHE_TIME:apache.error.timestamp}\] \[%{DATA:apache.error.module}:%{LOGLEVEL:log.level}\]
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

    - grok:
        field: source.address
        ignore_missing: true
        patterns:
        - ^(%{IP:source.ip}|%{HOSTNAME:source.domain})$
    - rename:
        field: source.as.organization_name
        target_field: source.as.organization.name
        ignore_missing: true
`),
			expected: []Processor{
				{Type: "grok", FirstLine: 4, LastLine: 11},
				{Type: "date", FirstLine: 12, LastLine: 21},
				{Type: "set", FirstLine: 22, LastLine: 26},
				{Type: "script", FirstLine: 27, LastLine: 29},
				{Type: "grok", FirstLine: 30, LastLine: 34},
				{Type: "rename", FirstLine: 35, LastLine: 38},
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
				{Type: "set", FirstLine: 4, LastLine: 8},
				{Type: "remove", FirstLine: 9, LastLine: 9},
				{Type: "set", FirstLine: 9, LastLine: 9},
				{Type: "set", FirstLine: 10, LastLine: 15},
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
			name:   "Json single line processor",
			format: "json",
			content: []byte(`{
				"description": "Pipeline for parsing silly logs.",
				"processors": [{"drop": {"if":"ctx.drop!=null"}}]
			  }`),
			expected: []Processor{
				{Type: "drop", FirstLine: 3, LastLine: 4},
			},
		},
		{
			name:   "Json multiline processor",
			format: "json",
			content: []byte(`{
  "processors": [
    {
      "script": {
        "description": "Extract fields",
        "lang": "painless",
        "source": "String[] envSplit = ctx['env'].splitOnToken(params['delimiter']);\nArrayList tags = new ArrayList();\ntags.add(envSplit[params['position']].trim());\nctx['tags'] = tags;"
      }
    }
  ]
}`),
			expected: []Processor{
				{Type: "script", FirstLine: 3, LastLine: 11},
				//   Source will be processed as multiline:
				//  "source": """
				// 		String[] envSplit = ctx['env'].splitOnToken(params['delimiter']);
				// 		ArrayList tags = new ArrayList();
				// 		tags.add(envSplit[params['position']].trim());
				// 		ctx['tags'] = tags;
				//   """,
			},
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
		{
			name:   "Yaml single line processor",
			format: "yml",
			content: []byte(`---
processors:
  - set: { field: "event.category", value: "web" }`),
			expected: []Processor{
				{Type: "set", FirstLine: 3, LastLine: 3},
			},
		},
		{
			name:   "Yaml multiline processor",
			format: "yml",
			content: []byte(`---
processors:
  - script:
      source: |
        def a = 1;
        def b = 2;
`),
			expected: []Processor{
				{Type: "script", FirstLine: 3, LastLine: 6},
			},
		},
		{
			name:   "Yaml script with empty line characters",
			format: "yml",
			content: []byte(`---
processors:
  - script:
      description: Do something.
      tag: script_drop_null_empty_values
      lang: painless
      source: "def a = b \n
	  ; def b = 2; \n"
`),
			expected: []Processor{
				{Type: "script", FirstLine: 3, LastLine: 8},
			},
		},
		{
			name:   "Yaml empty processor",
			format: "yml",
			content: []byte(`---
processors:
  - set:`),
			expected: []Processor{
				{Type: "set", FirstLine: 3, LastLine: 3},
			},
		},
		{
			name:   "Yaml processors with comments",
			format: "yml",
			content: []byte(`---
processors:
  # First processor
  - set:
      field: "event.category"
      value: "web"
  
  # Second processor
  - script:
      source: |
        def a = 1;
        def b = 2;
`),
			expected: []Processor{
				{Type: "set", FirstLine: 4, LastLine: 8},
				{Type: "script", FirstLine: 9, LastLine: 12},
			},
		},
		{
			name:   "Yaml nested processor",
			format: "yml",
			content: []byte(`---
processors:
  - if:
      condition: "ctx.event.category == 'web'"
      processors:
        - set: { field: "event.type", value: "start" }
`),
			expected: []Processor{
				{Type: "if", FirstLine: 3, LastLine: 6},
			},
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
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.expected, procs)
		})
	}
}
