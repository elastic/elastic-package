// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"encoding/json"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	estest "github.com/elastic/elastic-package/internal/elasticsearch/test"
	"github.com/elastic/elastic-package/internal/files"
)

func TestInstalledObjectsExportAll(t *testing.T) {
	client := estest.ElasticsearchClient(t, "./testdata/elasticsearch-mock-export-apache")
	outputDir := t.TempDir()
	exporter := NewInstalledObjectsExporter(client, "apache")
	n, err := exporter.ExportAll(context.Background(), outputDir)
	require.NoError(t, err)

	filesExpected := countFiles(t, "./testdata/apache-export-all")
	assert.Equal(t, filesExpected, n)

	filesFound := countFiles(t, outputDir)
	assert.Equal(t, filesExpected, filesFound)

	assertEqualExports(t, "./testdata/apache-export-all", outputDir)
}

func TestInstalledObjectsExportSome(t *testing.T) {
	client := estest.ElasticsearchClient(t, "./testdata/elasticsearch-mock-export-apache")
	exporter := NewInstalledObjectsExporter(client, "apache")

	// In a map so order of execution is randomized.
	exporters := map[string]func(ctx context.Context, outputDir string) (int, error){
		ComponentTemplatesExportDir: exporter.exportComponentTemplates,
		ILMPoliciesExportDir:        exporter.exportILMPolicies,
		IndexTemplatesExportDir:     exporter.exportIndexTemplates,
		IngestPipelinesExportDir:    exporter.exportIngestPipelines,
	}

	for dir, exportFunction := range exporters {
		t.Run(dir, func(t *testing.T) {
			outputDir := t.TempDir()
			n, err := exportFunction(context.Background(), outputDir)
			require.NoError(t, err)

			expectedDir := subDir(t, "./testdata/apache-export-all", dir)
			filesExpected := countFiles(t, expectedDir)
			assert.Equal(t, filesExpected, n)

			filesFound := countFiles(t, outputDir)
			assert.Equal(t, filesExpected, filesFound)

			assertEqualExports(t, expectedDir, outputDir)
		})
	}
}

func countFiles(t *testing.T, dir string) (count int) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		count++
		return nil
	})
	require.NoError(t, err)
	return count
}

func assertEqualExports(t *testing.T, expectedDir, resultDir string) {
	t.Helper()
	err := filepath.WalkDir(expectedDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		t.Run(path, func(t *testing.T) {
			relPath, err := filepath.Rel(expectedDir, path)
			require.NoError(t, err)

			assertEquivalentJSON(t, path, filepath.Join(resultDir, relPath))
		})
		return nil
	})
	require.NoError(t, err)
}

func assertEquivalentJSON(t *testing.T, expectedPath, foundPath string) {
	t.Helper()
	readJSON := func(p string) map[string]interface{} {
		d, err := ioutil.ReadFile(p)
		require.NoError(t, err)
		var o map[string]interface{}
		err = json.Unmarshal(d, &o)
		require.NoError(t, err)
		return o
	}

	expected := readJSON(expectedPath)
	found := readJSON(foundPath)
	assert.EqualValues(t, expected, found)
}

// subDir creates a temporary directory that contains a copy of a directory of the given directory. It returns
// the path of the temporary directory.
func subDir(t *testing.T, dir, name string) string {
	t.Helper()
	tmpDir := t.TempDir()

	dest := filepath.Join(tmpDir, name)
	src := filepath.Join(dir, name)

	os.MkdirAll(dest, 0755)
	err := files.CopyAll(src, dest)
	require.NoError(t, err)

	return tmpDir
}
