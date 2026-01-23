// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package reporters

// Reportable defines a raw report associated to a package.
type Reportable interface {
	Package() string
	Report() []byte
	WorkDir() string
}

var _ Reportable = &Report{}

type Report struct {
	pkg     string
	r       []byte
	workDir string
}

func NewReport(pkg, workDir string, p []byte) *Report {
	return &Report{pkg: pkg, r: p, workDir: workDir}
}

func (r *Report) Package() string { return r.pkg }

func (r *Report) Report() []byte { return r.r }

func (r *Report) WorkDir() string { return r.workDir }

// Reportable file associates a report to a filename.
type ReportableFile interface {
	Reportable
	Filename() string
}

var _ Reportable = &FileReport{}
var _ ReportableFile = &FileReport{}

type FileReport struct {
	pkg      string
	r        []byte
	filename string
	workDir  string
}

func NewFileReport(pkg, workDir, name string, p []byte) *FileReport {
	return &FileReport{pkg: pkg, r: p, filename: name, workDir: workDir}
}

func (r *FileReport) Package() string { return r.pkg }

func (r *FileReport) Report() []byte { return r.r }

func (r *FileReport) Filename() string { return r.filename }

func (r *FileReport) WorkDir() string { return r.workDir }

// MultiReportable defines an extended interface to ship multiple reports together.
// A call to Report() will return all reports contents combined.
type MultiReportable interface {
	Reportable
	Split() []Reportable
}

var _ Reportable = &MultiReport{}
var _ MultiReportable = &MultiReport{}

type MultiReport struct {
	pkg     string
	reports []Reportable
	workDir string
}

func NewMultiReport(pkg, workDir string, reports []Reportable) *MultiReport {
	return &MultiReport{pkg: pkg, reports: reports, workDir: workDir}
}

func (r *MultiReport) Package() string { return r.pkg }

func (r *MultiReport) WorkDir() string { return r.workDir }

func (r *MultiReport) Report() []byte {
	var combined []byte
	for _, fr := range r.reports {
		combined = append(combined, append(fr.Report(), '\n')...)
	}
	return combined
}

func (r *MultiReport) Split() []Reportable {
	return r.reports
}
