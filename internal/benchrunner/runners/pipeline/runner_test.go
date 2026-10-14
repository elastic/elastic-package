// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindBenchmarkFolders_PackageName(t *testing.T) {
	// Folder name (pipeline_benchmark) differs from the manifest name (pipeline_benchmarks).
	const packageRoot = "../../../../test/packages/benchmarks/pipeline_benchmark"
	const packageName = "pipeline_benchmarks"

	t.Run("all data streams", func(t *testing.T) {
		folders, err := FindBenchmarkFolders(packageRoot, nil)
		require.NoError(t, err)
		require.Len(t, folders, 1)

		assert.Equal(t, packageName, folders[0].Package)
		assert.Equal(t, "test", folders[0].DataStream)
	})

	t.Run("selected data streams", func(t *testing.T) {
		folders, err := FindBenchmarkFolders(packageRoot, []string{"test"})
		require.NoError(t, err)
		require.NotEmpty(t, folders)

		for _, folder := range folders {
			assert.Equal(t, packageName, folder.Package)
			assert.Equal(t, "test", folder.DataStream)
		}
	})

	t.Run("missing manifest", func(t *testing.T) {
		packageRoot := t.TempDir()
		benchmarkDir := filepath.Join(packageRoot, "data_stream", "logs", "_dev", "benchmark", "pipeline")
		require.NoError(t, os.MkdirAll(benchmarkDir, 0o755))

		_, err := FindBenchmarkFolders(packageRoot, nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "reading package manifest failed")
	})
}
