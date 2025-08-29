// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/dnaeon/go-vcr.v3/cassette"

	estest "github.com/elastic/elastic-package/internal/elasticsearch/test"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/stack"
)

func TestDumpInstalledObjects(t *testing.T) {
	// Files for each suite are recorded automatically on first test run.
	// To add a new suite:
	// - Configure it here.
	// - Install the package in a running stack.
	// - Configure environment variables for this stack (eval "$(elastic-package stack shellinit)").
	// - Run tests.
	// - Check that recorded files make sense and commit them.
	// To update a suite:
	// - Reproduce the scenario as described in the comments.
	// - Remove the files that you want to update.
	// - Follow the same steps to create a new suite.
	// - Check if the changes are the expected ones and commit them.
	suites := []*installedObjectsDumpSuite{
		&installedObjectsDumpSuite{
			// To reproduce the scenario:
			// - Start the stack with version 7.16.2.
			// - Install apache package (1.3.5).
			PackageName: "apache",
			Record:      "./testdata/elasticsearch-7-mock-dump-apache",
			DumpDir:     "./testdata/elasticsearch-7-apache-dump-all",
			Matcher:     ingestPipelineRequestMatcher,
		},
		&installedObjectsDumpSuite{
			// To reproduce the scenario:
			// - Start the stack with version 8.1.0.
			// - Install apache package (1.8.2).
			PackageName: "apache",
			Record:      "./testdata/elasticsearch-8-mock-dump-apache",
			DumpDir:     "./testdata/elasticsearch-8-apache-dump-all",
			Matcher:     ingestPipelineRequestMatcher,
		},
		&installedObjectsDumpSuite{
			// To reproduce the scenario:
			// - Start the stack with version 8.9.0.
			// - Install dga package (2.1.0).
			// - Manually replace the `compressed_definition` fields with "//REDACTED//".
			PackageName: "dga",
			Record:      "./testdata/elasticsearch-8-mock-dump-dga",
			DumpDir:     "./testdata/elasticsearch-8-dga-dump-all",
			Matcher:     ingestPipelineRequestMatcher,
		},
	}

	for _, s := range suites {
		suite.Run(t, s)
	}
}

type installedObjectsDumpSuite struct {
	suite.Suite

	// PackageName is the name of the package.
	PackageName string

	// Record is where responses from Elasticsearch are recorded.
	Record string

	// DumpDir is where the expected dumped files are stored.
	DumpDir string

	// Function that helps match an outbound request to a recorded one
	Matcher cassette.MatcherFunc
}

func (s *installedObjectsDumpSuite) SetupTest() {
	_, err := os.Stat(s.DumpDir)
	if errors.Is(err, os.ErrNotExist) {
		client, err := stack.NewElasticsearchClient()
		s.Require().NoError(err)

		dumper := NewInstalledObjectsDumper(client.API, s.PackageName)
		n, err := dumper.DumpAll(s.T().Context(), s.DumpDir)
		s.Require().NoError(err)
		s.Require().Greater(n, 0)
	} else {
		s.Require().NoError(err)
	}
}

func (s *installedObjectsDumpSuite) TestDumpAll() {
	client := estest.NewClient(s.T(), s.Record, s.Matcher)

	outputDir := s.T().TempDir()
	dumper := NewInstalledObjectsDumper(client.API, s.PackageName)
	n, err := dumper.DumpAll(s.T().Context(), outputDir)
	s.Require().NoError(err)

	filesExpected := countFiles(s.T(), s.DumpDir)
	s.Assert().Equal(filesExpected, n)

	filesFound := countFiles(s.T(), outputDir)
	s.Assert().Equal(filesExpected, filesFound)

	assertEqualDumps(s.T(), s.DumpDir, outputDir)
}

func (s *installedObjectsDumpSuite) TestDumpSome() {
	client := estest.NewClient(s.T(), s.Record, s.Matcher)
	dumper := NewInstalledObjectsDumper(client.API, s.PackageName)

	// In a map so order of execution is randomized.
	dumpers := map[string]func(ctx context.Context, outputDir string) (int, error){
		ComponentTemplatesDumpDir: dumper.dumpComponentTemplates,
		ILMPoliciesDumpDir:        dumper.dumpILMPolicies,
		IndexTemplatesDumpDir:     dumper.dumpIndexTemplates,
		IngestPipelinesDumpDir:    dumper.dumpIngestPipelines,
		MLModelsDumpDir:           dumper.dumpMLModels,
	}

	for dir, dumpFunction := range dumpers {
		s.Run(dir, func() {
			outputDir := s.T().TempDir()
			n, err := dumpFunction(s.T().Context(), outputDir)
			s.Require().NoError(err)

			expectedDir := subDir(s.T(), s.DumpDir, dir)
			filesExpected := countFiles(s.T(), expectedDir)
			s.Assert().Equal(filesExpected, n)

			filesFound := countFiles(s.T(), outputDir)
			s.Assert().Equal(filesExpected, filesFound)

			assertEqualDumps(s.T(), expectedDir, outputDir)
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
		d, err := os.ReadFile(p)
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
	os.MkdirAll(dest, 0755)

	src := filepath.Join(dir, name)
	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		// Not all packages have all kinds of objects.
		return tmpDir
	}

	err := files.CopyAll(src, dest)
	require.NoError(t, err)

	return tmpDir
}

// Ingest Pipelines are requested in bulk and the param values are non-deterministic,
// which makes matching to recorded requests flaky.
// This custom cassette matcher helps match pipeline requests, and sends all others to the default matcher.
func ingestPipelineRequestMatcher(r *http.Request, cr cassette.Request) bool {
	urlStartPattern := "https://127.0.0.1:9200/_ingest/pipeline/"
	rSplitUrl := strings.Split(r.URL.String(), urlStartPattern)
	crSplitUrl := strings.Split(cr.URL, urlStartPattern)

	isURLsPattern := len(rSplitUrl) == 2 && len(crSplitUrl) == 2

	if !isURLsPattern {
		return cassette.DefaultMatcher(r, cr)
	}

	rPipelineValues := strings.Split(rSplitUrl[1], ",")
	crPipelineValues := strings.Split(crSplitUrl[1], ",")

	slices.Sort(rPipelineValues)
	slices.Sort(crPipelineValues)

	return slices.Equal(rPipelineValues, crPipelineValues)
}
