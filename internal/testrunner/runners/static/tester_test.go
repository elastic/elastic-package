// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/testrunner"
)

func TestGetSampleEventPaths(t *testing.T) {
	repositoryRoot, err := files.FindRepositoryRoot()
	require.NoError(t, err)
	t.Cleanup(func() { _ = repositoryRoot.Close() })

	otelPackageRoot := filepath.Join(repositoryRoot.Name(), "test", "packages", "parallel", "sql_server_input_otel")

	cases := []struct {
		title         string
		packageRoot   string
		dataStream    string
		expectedNames []string
	}{
		{
			title:       "OTel input package with type-qualified sample events",
			packageRoot: otelPackageRoot,
			expectedNames: []string{
				"sample_event.logs.json",
				"sample_event.metrics.json",
			},
		},
		{
			title:         "Package root with no sample event files",
			packageRoot:   t.TempDir(),
			expectedNames: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			r := tester{
				packageRoot: c.packageRoot,
				testFolder: testrunner.TestFolder{
					DataStream: c.dataStream,
				},
			}

			paths, err := r.getSampleEventPaths()
			require.NoError(t, err)

			var names []string
			for _, p := range paths {
				names = append(names, filepath.Base(p))
			}
			assert.Equal(t, c.expectedNames, names)
		})
	}
}
