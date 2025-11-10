// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"os"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/testrunner"
)

// Options contains benchmark runner options.
type Options struct {
	BenchName       string
	Folder          testrunner.TestFolder
	PackageRootPath string
	API             *elasticsearch.API
	NumTopProcs     int
	Format          Format
	RepositoryRoot  *os.Root
}

type OptionFunc func(*Options)

func NewOptions(fns ...OptionFunc) Options {
	var opts Options
	for _, fn := range fns {
		fn(&opts)
	}
	return opts
}

func WithFolder(f testrunner.TestFolder) OptionFunc {
	return func(opts *Options) {
		opts.Folder = f
	}
}

func WithPackageRootPath(path string) OptionFunc {
	return func(opts *Options) {
		opts.PackageRootPath = path
	}
}

func WithESAPI(api *elasticsearch.API) OptionFunc {
	return func(opts *Options) {
		opts.API = api
	}
}

func WithNumTopProcs(n int) OptionFunc {
	return func(opts *Options) {
		opts.NumTopProcs = n
	}
}

func WithFormat(format string) OptionFunc {
	return func(opts *Options) {
		opts.Format = Format(format)
	}
}

func WithBenchmarkName(name string) OptionFunc {
	return func(opts *Options) {
		opts.BenchName = name
	}
}

func WithRepositoryRoot(root *os.Root) OptionFunc {
	return func(opts *Options) {
		opts.RepositoryRoot = root
	}
}
