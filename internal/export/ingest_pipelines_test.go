// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
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
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/stack"

	estest "github.com/elastic/elastic-package/internal/elasticsearch/test"
)

func TestExportIngestPipelines(t *testing.T) {
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
	suites := []*ingestPipelineExportSuite{
		// To reproduce the scenario:
		// - Start the stack with version 8.17.4.
		// - Install apache package (2.0.0).
		// - Install dga package (2.3.0).
		&ingestPipelineExportSuite{
			PipelineIds: []string{
				"logs-apache.access-2.0.0",
				"2.3.0-ml_dga_ingest_pipeline",
			},
			ExportDir: "./testdata/elasticsearch-8-export-pipelines",
			RecordDir: "./testdata/elasticsearch-8-mock-export-pipelines",
			Matcher:   ingestPipelineRequestMatcher,
		},
	}

	for _, s := range suites {
		suite.Run(t, s)
	}
}

type ingestPipelineExportSuite struct {
	suite.Suite

	PipelineIds []string
	ExportDir   string
	RecordDir   string
	Matcher     cassette.MatcherFunc
}

func (s *ingestPipelineExportSuite) SetupTest() {
	_, err := os.Stat(s.ExportDir)
	if errors.Is(err, os.ErrNotExist) {
		client, err := stack.NewElasticsearchClient()
		s.Require().NoError(err)

		writeAssignments := createTestWriteAssignments(s.PipelineIds, s.ExportDir)

		err = IngestPipelines(s.T().Context(), client.API, writeAssignments)

		s.Require().NoError(err)
	} else {
		s.Require().NoError(err)
	}
}

func (s *ingestPipelineExportSuite) TestExportPipelines() {
	client := estest.NewClient(s.T(), s.RecordDir, s.Matcher)

	outputDir := s.T().TempDir()
	writeAssignments := createTestWriteAssignments(s.PipelineIds, outputDir)
	err := IngestPipelines(s.T().Context(), client.API, writeAssignments)
	s.Require().NoError(err)

	filesExpected := countFiles(s.T(), s.ExportDir)
	filesExported := countFiles(s.T(), outputDir)
	s.Assert().Equal(filesExpected, filesExported)

	filesFound := countFiles(s.T(), outputDir)
	s.Assert().Equal(filesExpected, filesFound)

	assertEqualExports(s.T(), s.ExportDir, outputDir)
}

func createTestWriteAssignments(pipelineIDs []string, outputDir string) PipelineWriteAssignments {
	writeAssignments := make(PipelineWriteAssignments)

	for _, pipelineID := range pipelineIDs {
		writeAssignments[pipelineID] = PipelineWriteLocation{
			Type:       PipelineWriteLocationTypeRoot,
			Name:       pipelineID,
			ParentPath: outputDir,
		}
	}

	return writeAssignments
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

			assertEquivalentYAML(t, path, filepath.Join(resultDir, relPath))
		})
		return nil
	})
	require.NoError(t, err)
}

func assertEquivalentYAML(t *testing.T, expectedPath, foundPath string) {
	t.Helper()
	readYAML := func(p string) map[string]interface{} {
		d, err := os.ReadFile(p)
		require.NoError(t, err)
		var o map[string]interface{}
		err = yaml.Unmarshal(d, &o)
		require.NoError(t, err)
		return o
	}

	expected := readYAML(expectedPath)
	found := readYAML(foundPath)
	assert.EqualValues(t, expected, found)
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
