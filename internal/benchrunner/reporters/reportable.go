// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package reporters

type Reportable interface {
	Package() string
	Report() []byte
}

type Report struct {
	benchName string
	p         []byte
}

func NewReport(benchName string, p []byte) *Report {
	return &Report{benchName: benchName, p: p}
}

func (r *Report) Package() string { return r.benchName }

func (r *Report) Report() []byte { return r.p }

type ReportableFile interface {
	Reportable
	Extension() string
}

type FileReport struct {
	benchName string
	p         []byte
	extension string
}

func NewFileReport(benchName, ext string, p []byte) *FileReport {
	return &FileReport{benchName: benchName, p: p, extension: ext}
}

func (r *FileReport) Package() string { return r.benchName }

func (r *FileReport) Report() []byte { return r.p }

func (r *FileReport) Extension() string { return r.extension }
