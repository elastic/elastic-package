// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestDefaultExpectedDatasets(t *testing.T) {
	cases := []struct {
		title      string
		pkgName    string
		dataStream string
		manifest   *packages.DataStreamManifest
		expected   []string
	}{
		{
			title:      "non-otelcol: uses package.datastream as dataset",
			pkgName:    "nginx",
			dataStream: "access",
			manifest: &packages.DataStreamManifest{
				Streams: []packages.Stream{{Input: "logfile"}},
			},
			expected: []string{"nginx.access"},
		},
		{
			title:      "non-otelcol: explicit dataset in manifest",
			pkgName:    "nginx",
			dataStream: "access",
			manifest: &packages.DataStreamManifest{
				Dataset: "custom_dataset",
				Streams: []packages.Stream{{Input: "logfile"}},
			},
			expected: []string{"custom_dataset"},
		},
		{
			title:      "otelcol: base and .otel suffix",
			pkgName:    "claude_code",
			dataStream: "events",
			manifest: &packages.DataStreamManifest{
				Streams: []packages.Stream{{Input: "otelcol"}},
			},
			expected: []string{"claude_code.events", "claude_code.events.otel"},
		},
		{
			title:      "otelcol: explicit dataset in manifest with .otel suffix",
			pkgName:    "mypackage",
			dataStream: "logs",
			manifest: &packages.DataStreamManifest{
				Dataset: "custom",
				Streams: []packages.Stream{{Input: "otelcol"}},
			},
			expected: []string{"custom", "custom.otel"},
		},
		{
			title:      "otelcol among other inputs: .otel suffix added",
			pkgName:    "mypackage",
			dataStream: "events",
			manifest: &packages.DataStreamManifest{
				Streams: []packages.Stream{
					{Input: "logfile"},
					{Input: "otelcol"},
				},
			},
			expected: []string{"mypackage.events", "mypackage.events.otel"},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			got := defaultExpectedDatasets(c.pkgName, c.dataStream, c.manifest)
			assert.Equal(t, c.expected, got)
		})
	}
}

func TestIsOTelCollectorInput(t *testing.T) {
	cases := []struct {
		title    string
		manifest *packages.DataStreamManifest
		expected bool
	}{
		{
			title: "no streams",
			manifest: &packages.DataStreamManifest{
				Streams: nil,
			},
			expected: false,
		},
		{
			title: "logfile input only",
			manifest: &packages.DataStreamManifest{
				Streams: []packages.Stream{
					{Input: "logfile"},
				},
			},
			expected: false,
		},
		{
			title: "multiple non-otelcol inputs",
			manifest: &packages.DataStreamManifest{
				Streams: []packages.Stream{
					{Input: "logfile"},
					{Input: "httpjson"},
					{Input: "cel"},
				},
			},
			expected: false,
		},
		{
			title: "otelcol input only",
			manifest: &packages.DataStreamManifest{
				Streams: []packages.Stream{
					{Input: "otelcol"},
				},
			},
			expected: true,
		},
		{
			title: "otelcol among other inputs",
			manifest: &packages.DataStreamManifest{
				Streams: []packages.Stream{
					{Input: "logfile"},
					{Input: "otelcol"},
				},
			},
			expected: true,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			got := isOTelCollectorInput(c.manifest)
			assert.Equal(t, c.expected, got)
		})
	}
}
