// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchmark

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/benchrunner/runners/pipeline"
	"github.com/elastic/elastic-package/internal/reportgenerator"
)

const (
	// ReportType defining benchmark reports.
	ReportType reportgenerator.ReportType = "benchmark"
)

type Report struct {
	Package    string
	DataStream string
	Old        float64
	New        float64
	Diff       float64
	Percentage float64
}

type Reports map[string][]Report

type generator struct {
	options reportgenerator.ReportOptions
}

// Type returns the type of benchmark that can be run by this benchmark runner.
func (*generator) Type() reportgenerator.ReportType {
	return ReportType
}

// String returns the human-friendly name of the benchmark runner.
func (*generator) String() string {
	return "benchmark"
}

// Format returns the format used by the report.
func (*generator) Format() string {
	return "md"
}

// Run runs the pipeline benchmarks defined under the given folder
func (g *generator) Generate(options reportgenerator.ReportOptions) ([]byte, error) {
	g.options = options
	return g.generate()
}

func (g *generator) generate() ([]byte, error) {
	// get all results from new
	newResults, err := listAllDirResults(g.options.NewPath)
	if err != nil {
		return nil, fmt.Errorf("listing new results failed: %w", err)
	}

	// get all results from old
	oldResults, err := listAllDirResultsAsMap(g.options.OldPath)
	if err != nil {
		return nil, fmt.Errorf("listing old results failed: %w", err)
	}

	// lookup new reports in the old ones and compare
	reports := Reports{}
	for _, entry := range newResults {
		newRes, err := readResult(g.options.NewPath, entry)
		if err != nil {
			return nil, fmt.Errorf("reading new result: %w", err)
		}
		pkg, ds := newRes.Package, newRes.DataStream
		var oldRes pipeline.BenchmarkResult
		if oldEntry, found := oldResults[pkg]; found {
			if ds, found := oldEntry[ds]; found {
				oldRes, err = readResult(g.options.OldPath, ds)
				if err != nil {
					return nil, fmt.Errorf("reading old result: %w", err)
				}
			}
		}
		report := createReport(newRes, oldRes)
		reports[report.Package] = append(reports[report.Package], report)
	}

	return g.markdownFormat(reports)
}

func createReport(new, old pipeline.BenchmarkResult) Report {
	var r Report
	r.Package, r.DataStream = new.Package, new.DataStream

	// we round all the values to 2 decimals approximations
	r.New = roundFloat64(getEPS(new))
	r.Old = roundFloat64(getEPS(old))
	r.Diff = roundFloat64(r.New - r.Old)
	if r.Old > 0 {
		r.Percentage = roundFloat64((r.Diff / r.Old) * 100)
	}

	return r
}

func getEPS(r pipeline.BenchmarkResult) float64 {
	for _, test := range r.Tests {
		for _, res := range test.Results {
			if res.Name == "eps" {
				v, _ := res.Value.(float64)
				return v
			}
		}
	}
	return 0
}

func roundFloat64(v float64) float64 {
	return math.Round(v*100) / 100
}

func listAllDirResults(path string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory failed (path: %s): %w", path, err)
	}

	// only keep results, scan is not recursive
	var filtered []os.DirEntry
	for _, e := range entries {
		if e.IsDir() || !resultExts[filepath.Ext(e.Name())] {
			continue
		}
		filtered = append(filtered, e)
	}

	return filtered, nil
}

func listAllDirResultsAsMap(path string) (map[string]map[string]fs.DirEntry, error) {
	entries, err := listAllDirResults(path)
	if err != nil {
		return nil, err
	}

	m := map[string]map[string]fs.DirEntry{}
	for _, entry := range entries {
		res, err := readResult(path, entry)
		if err != nil {
			return nil, fmt.Errorf("reading result: %w", err)
		}
		pkg, ds := res.Package, res.DataStream
		if m[pkg] == nil {
			m[pkg] = map[string]fs.DirEntry{}
		}
		m[pkg][ds] = entry
	}

	return m, nil
}

func readResult(path string, e fs.DirEntry) (pipeline.BenchmarkResult, error) {
	fi, err := e.Info()
	if err != nil {
		return pipeline.BenchmarkResult{}, fmt.Errorf("getting file info failed (file: %s): %w", e.Name(), err)
	}

	b, err := os.ReadFile(path + string(os.PathSeparator) + fi.Name())
	if err != nil {
		return pipeline.BenchmarkResult{}, fmt.Errorf("reading result contents (file: %s): %w", fi.Name(), err)
	}

	var br pipeline.BenchmarkResult
	switch ext := filepath.Ext(fi.Name()); ext {
	case ".json":
		if err := json.Unmarshal(b, &br); err != nil {
			return pipeline.BenchmarkResult{}, fmt.Errorf("decoding json (file: %s): %w", fi.Name(), err)
		}
	case ".xml":
		if err := xml.Unmarshal(b, &br); err != nil {
			return pipeline.BenchmarkResult{}, fmt.Errorf("decoding xml (file: %s): %w", fi.Name(), err)
		}
	default:
		return pipeline.BenchmarkResult{}, fmt.Errorf("unsupported result format: %v", ext)
	}

	return br, nil
}

func init() {
	reportgenerator.RegisterGenerator(&generator{})
}
