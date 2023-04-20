// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
)

// Options contains benchmark runner options.
type Options struct {
	ESAPI            *elasticsearch.API
	KibanaClient     *kibana.Client
	MetricstoreESURL string
	BenchName        string
	PackageRootPath  string
}

type OptionFunc func(*Options)

func NewOptions(fns ...OptionFunc) Options {
	var opts Options
	for _, fn := range fns {
		fn(&opts)
	}
	return opts
}

func WithESAPI(api *elasticsearch.API) OptionFunc {
	return func(opts *Options) {
		opts.ESAPI = api
	}
}

func WithKibanaClient(c *kibana.Client) OptionFunc {
	return func(opts *Options) {
		opts.KibanaClient = c
	}
}

func WithPackageRootPath(path string) OptionFunc {
	return func(opts *Options) {
		opts.PackageRootPath = path
	}
}

func WithBenchmarkName(name string) OptionFunc {
	return func(opts *Options) {
		opts.BenchName = name
	}
}
