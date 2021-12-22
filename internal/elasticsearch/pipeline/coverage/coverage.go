// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package coverage

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/elasticsearch/node_stats"
	"github.com/elastic/elastic-package/internal/elasticsearch/pipeline"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

// Get returns a coverage report for the provided set of ingest pipelines.
func Get(options testrunner.TestOptions, pipelines []pipeline.Resource) (*testrunner.CoberturaCoverage, error) {
	packagePath, err := packages.MustFindPackageRoot()
	if err != nil {
		return nil, errors.Wrap(err, "error finding package root")
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(options.TestFolder.Path)
	if err != nil {
		return nil, errors.Wrap(err, "locating data_stream root failed")
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	// Use the Node Stats API to get stats for all installed pipelines.
	// These stats contain hit counts for all main processors in a pipeline.
	stats, err := node_stats.GetPipelineStats(options.API, pipelines)
	if err != nil {
		return nil, errors.Wrap(err, "error fetching pipeline stats for code coverage calculations")
	}

	// Construct the Cobertura report.
	pkg := &testrunner.CoberturaPackage{
		Name: options.TestFolder.Package + "." + options.TestFolder.DataStream,
	}

	coverage := &testrunner.CoberturaCoverage{
		Sources: []*testrunner.CoberturaSource{
			{
				Path: packagePath,
			},
		},
		Packages:  []*testrunner.CoberturaPackage{pkg},
		Timestamp: time.Now().UnixNano(),
	}

	for _, p := range pipelines {
		// Load the list of main processors from the pipeline source code, annotated with line numbers.
		src, err := p.Processors()
		if err != nil {
			return nil, err
		}

		pstats := stats[p.Name]
		if pstats == nil {
			return nil, errors.Errorf("pipeline '%s' not installed in Elasticsearch", p.Name)
		}

		// Ensure there is no inconsistency in the list of processors in stats vs obtained from source.
		if len(src) != len(pstats.Processors) {
			return nil, errors.Errorf("processor count mismatch for %s (src:%d stats:%d)", p.FileName(), len(src), len(pstats.Processors))
		}
		for idx, st := range pstats.Processors {
			// Elasticsearch will return a `compound` processor in the case of `foreach` and
			// any processor that defines `on_failure` processors.
			if st.Type == "compound" {
				continue
			}
			if st.Type != src[idx].Type {
				return nil, errors.Errorf("processor type mismatch for %s processor %d (src:%s stats:%s)", p.FileName(), idx, src[idx].Type, st.Type)
			}
		}
		// Tests install pipelines as `filename-<nonce>` (without original extension).
		// Use the filename part for the report.
		pipelineName := p.Name
		if nameEnd := strings.LastIndexByte(pipelineName, '-'); nameEnd != -1 {
			pipelineName = pipelineName[:nameEnd]
		}
		// File path has to be relative to the packagePath added to the cobertura Sources list.
		pipelinePath := filepath.Join(dataStreamPath, "elasticsearch", "ingest_pipeline", p.FileName())
		pipelineRelPath, err := filepath.Rel(packagePath, pipelinePath)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot create relative path to pipeline file. Package root: '%s', pipeline path: '%s'", packagePath, pipelinePath)
		}
		// Report every pipeline as a "class".
		classCov := testrunner.CoberturaClass{
			Name:     pipelineName,
			Filename: pipelineRelPath,
		}

		// Calculate covered and total processors (reported as both lines and methods).
		covered := 0
		for idx, srcProc := range src {
			if pstats.Processors[idx].Stats.Count > 0 {
				covered++
			}
			method := testrunner.CoberturaMethod{
				Name: srcProc.Type,
				Hits: pstats.Processors[idx].Stats.Count,
			}
			for num := srcProc.FirstLine; num <= srcProc.LastLine; num++ {
				line := &testrunner.CoberturaLine{
					Number: num,
					Hits:   pstats.Processors[idx].Stats.Count,
				}
				classCov.Lines = append(classCov.Lines, line)
				method.Lines = append(method.Lines, line)
			}
			classCov.Methods = append(classCov.Methods, &method)
		}
		pkg.Classes = append(pkg.Classes, &classCov)
		coverage.LinesValid += int64(len(src))
		coverage.LinesCovered += int64(covered)
	}
	return coverage, nil
}
