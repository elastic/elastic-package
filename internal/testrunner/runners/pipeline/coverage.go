// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

// getPipelineCoverage returns a coverage report for the provided set of ingest pipelines.
func getPipelineCoverage(pkgName string, options PipelineTesterOptions, pipelines []ingest.Pipeline) (testrunner.CoverageReport, error) {
	dataStreamRoot, found, err := packages.FindDataStreamRootForPath(options.TestFolder.Path)
	if err != nil {
		return nil, fmt.Errorf("locating data_stream root failed: %w", err)
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	// Use the Node Stats API to get stats for all installed pipelines.
	// These stats contain hit counts for all main processors in a pipeline.
	stats, err := ingest.GetPipelineStats(options.API, pipelines)
	if err != nil {
		return nil, fmt.Errorf("error fetching pipeline stats for code coverage calculations: %w", err)
	}

	// Use the package's parent directory as base path, so that the relative paths
	// for each class (pipeline) include the package name. This prevents paths for
	// different packages colliding (i.e. a lot of packages have a "log" datastream
	// and a default.yml pipeline).
	basePath := filepath.Dir(options.PackageRoot)

	repositoryRoot, err := files.FindRepositoryRoot()
	if err != nil {
		return nil, err
	}
	defer repositoryRoot.Close()

	if options.CoverageType == "cobertura" {
		pkg := &testrunner.CoberturaPackage{
			Name: pkgName,
		}

		cobertura := &testrunner.CoberturaCoverage{
			Sources: []*testrunner.CoberturaSource{
				{
					Path: basePath,
				},
			},
			Packages:  []*testrunner.CoberturaPackage{pkg},
			Timestamp: time.Now().UnixNano(),
		}

		// Calculate coverage for each pipeline
		for _, pipeline := range pipelines {
			pipelineName, pipelineRelPath, src, pstats, err := pipelineDataForCoverage(pipeline, stats, repositoryRoot.Name(), dataStreamRoot)
			if err != nil {
				return nil, err
			}
			covered, class, err := coberturaForSinglePipeline(pipelineName, pipelineRelPath, src, pstats)
			if err != nil {
				return nil, fmt.Errorf("error calculating coverage for pipeline '%s': %w", pipeline.Filename(), err)
			}
			pkg.Classes = append(pkg.Classes, class)
			cobertura.LinesValid += int64(len(class.Methods))
			cobertura.LinesCovered += covered
		}
		return cobertura, nil
	}

	if options.CoverageType == "generic" {
		coverage := &testrunner.GenericCoverage{
			Version:   1,
			Timestamp: time.Now().UnixNano(),
			TestType:  "Cobertura for pipeline test",
		}

		// Calculate coverage for each pipeline
		for _, pipeline := range pipelines {
			_, pipelineRelPath, src, pstats, err := pipelineDataForCoverage(pipeline, stats, repositoryRoot.Name(), dataStreamRoot)
			if err != nil {
				return nil, err
			}
			_, file, err := genericCoverageForSinglePipeline(pipelineRelPath, src, pstats)
			if err != nil {
				return nil, fmt.Errorf("error calculating coverage for pipeline '%s': %w", pipeline.Filename(), err)
			}
			coverage.Files = append(coverage.Files, file)
		}
		return coverage, nil

	}

	return nil, fmt.Errorf("unrecognised coverage type")
}

func pipelineDataForCoverage(pipeline ingest.Pipeline, stats ingest.PipelineStatsMap, basePath, dataStreamRoot string) (string, string, []ingest.Processor, ingest.PipelineStats, error) {
	// Load the list of main processors from the pipeline source code, annotated with line numbers.
	src, err := pipeline.OriginalProcessors()
	if err != nil {
		return "", "", nil, ingest.PipelineStats{}, err
	}

	pstats, found := stats[pipeline.Name]
	if !found {
		return "", "", nil, ingest.PipelineStats{}, fmt.Errorf("pipeline '%s' not installed in Elasticsearch", pipeline.Name)
	}

	// Remove reroute processors if any so the pipeline has the same processors as in the file
	// reroute processors are added if there are any routing_rules file defined
	var processors []ingest.ProcessorStats
	for _, proc := range pstats.Processors {
		if proc.Type == "reroute" {
			continue
		}
		processors = append(processors, proc)
	}
	pstats.Processors = processors

	// Ensure there is no inconsistency in the list of processors in stats vs obtained from source.
	if len(src) != len(pstats.Processors) {
		return "", "", nil, ingest.PipelineStats{}, fmt.Errorf("processor count mismatch for %s (src:%d stats:%d)", pipeline.Filename(), len(src), len(pstats.Processors))
	}
	for idx, st := range pstats.Processors {
		// Check that we have the expected type of processor, except for `compound` processors.
		// Elasticsearch will return a `compound` processor in the case of `foreach` and
		// any processor that defines `on_failure` processors.
		if st.Type != "compound" && st.Type != src[idx].Type {
			return "", "", nil, ingest.PipelineStats{}, fmt.Errorf("processor type mismatch for %s processor %d (src:%s stats:%s)", pipeline.Filename(), idx, src[idx].Type, st.Type)
		}
	}

	// Tests install pipelines as `filename-<nonce>` (without original extension).
	// Use the filename part for the report.
	pipelineName := pipeline.Name
	if nameEnd := strings.LastIndexByte(pipelineName, '-'); nameEnd != -1 {
		pipelineName = pipelineName[:nameEnd]
	}

	// File path has to be relative to the packagePath added to the cobertura Sources list
	// so that the source is reachable by the report tool.
	pipelinePath := filepath.Join(dataStreamRoot, "elasticsearch", "ingest_pipeline", pipeline.Filename())
	pipelineRelPath, err := filepath.Rel(basePath, pipelinePath)
	if err != nil {
		return "", "", nil, ingest.PipelineStats{}, fmt.Errorf("cannot create relative path to pipeline file. Package root: '%s', pipeline path: '%s': %w", basePath, pipelinePath, err)
	}

	return pipelineName, pipelineRelPath, src, pstats, nil
}

func genericCoverageForSinglePipeline(pipelineRelPath string, src []ingest.Processor, pstats ingest.PipelineStats) (linesCovered int64, class *testrunner.GenericFile, err error) {
	// Report every pipeline as a "file".
	file := &testrunner.GenericFile{
		Path: pipelineRelPath,
	}
	for idx, srcProc := range src {
		if pstats.Processors[idx].Stats.Count > 0 {
			linesCovered++
		}
		for num := srcProc.FirstLine; num <= srcProc.LastLine; num++ {
			line := &testrunner.GenericLine{
				LineNumber: int64(num),
				Covered:    pstats.Processors[idx].Stats.Count > 0,
			}
			file.Lines = append(file.Lines, line)
		}
	}
	return linesCovered, file, nil
}

func coberturaForSinglePipeline(pipelineName, pipelineRelPath string, src []ingest.Processor, pstats ingest.PipelineStats) (linesCovered int64, class *testrunner.CoberturaClass, err error) {
	// Report every pipeline as a "class".
	class = &testrunner.CoberturaClass{
		Name:     pipelineName,
		Filename: pipelineRelPath,
	}

	// Calculate covered and total processors (reported as both lines and methods).
	for idx, srcProc := range src {
		if pstats.Processors[idx].Stats.Count > 0 {
			linesCovered++
		}
		method := testrunner.CoberturaMethod{
			Name: srcProc.Type,
		}
		for num := srcProc.FirstLine; num <= srcProc.LastLine; num++ {
			line := &testrunner.CoberturaLine{
				Number: num,
				Hits:   pstats.Processors[idx].Stats.Count,
			}
			class.Lines = append(class.Lines, line)
			method.Lines = append(method.Lines, line)
		}
		class.Methods = append(class.Methods, &method)
	}
	return linesCovered, class, nil
}
