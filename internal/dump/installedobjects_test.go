// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

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

func TestInstalledObjectsDumpAll(t *testing.T) {
	client := estest.ElasticsearchClient(t, "./testdata/elasticsearch-mock-dump-apache")
	outputDir := t.TempDir()
	dumper := NewInstalledObjectsDumper(client, "apache")
	n, err := dumper.DumpAll(context.Background(), outputDir)
	require.NoError(t, err)

	filesExpected := countFiles(t, "./testdata/apache-dump-all")
	assert.Equal(t, filesExpected, n)

	filesFound := countFiles(t, outputDir)
	assert.Equal(t, filesExpected, filesFound)

	assertEqualDumps(t, "./testdata/apache-dump-all", outputDir)
}

func TestInstalledObjectsDumpSome(t *testing.T) {
	client := estest.ElasticsearchClient(t, "./testdata/elasticsearch-mock-dump-apache")
	dumper := NewInstalledObjectsDumper(client, "apache")

	// In a map so order of execution is randomized.
	dumpers := map[string]func(ctx context.Context, outputDir string) (int, error){
		ComponentTemplatesDumpDir: dumper.dumpComponentTemplates,
		ILMPoliciesDumpDir:        dumper.dumpILMPolicies,
		IndexTemplatesDumpDir:     dumper.dumpIndexTemplates,
		IngestPipelinesDumpDir:    dumper.dumpIngestPipelines,
	}

	for dir, dumpFunction := range dumpers {
		t.Run(dir, func(t *testing.T) {
			outputDir := t.TempDir()
			n, err := dumpFunction(context.Background(), outputDir)
			require.NoError(t, err)

			expectedDir := subDir(t, "./testdata/apache-dump-all", dir)
			filesExpected := countFiles(t, expectedDir)
			assert.Equal(t, filesExpected, n)

			filesFound := countFiles(t, outputDir)
			assert.Equal(t, filesExpected, filesFound)

			assertEqualDumps(t, expectedDir, outputDir)
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

func assertEqualDumps(t *testing.T, expectedDir, resultDir string) {
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
