// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package rally

import (
	"os"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/profile"
)

// Options contains benchmark runner options.
type Options struct {
	ESAPI               *elasticsearch.API
	KibanaClient        *kibana.Client
	DeferCleanup        time.Duration
	MetricsInterval     time.Duration
	ReindexData         bool
	ESMetricsAPI        *elasticsearch.API
	BenchName           string
	PackageRootPath     string
	Variant             string
	Profile             *profile.Profile
	RallyTrackOutputDir string
	DryRun              bool
	PackageName         string
	PackageVersion      string
	CorpusAtPath        string
	RepositoryRoot      *os.Root
}

type ClientOptions struct {
	Host     string
	Username string
	Password string
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

func WithDataReindexing(b bool) OptionFunc {
	return func(opts *Options) {
		opts.ReindexData = b
	}
}

func WithESMetricsAPI(api *elasticsearch.API) OptionFunc {
	return func(opts *Options) {
		opts.ESMetricsAPI = api
	}
}

func WithVariant(name string) OptionFunc {
	return func(opts *Options) {
		opts.Variant = name
	}
}

func WithProfile(p *profile.Profile) OptionFunc {
	return func(opts *Options) {
		opts.Profile = p
	}
}

func WithRallyTrackOutputDir(r string) OptionFunc {
	return func(opts *Options) {
		opts.RallyTrackOutputDir = r
	}
}

func WithRallyDryRun(d bool) OptionFunc {
	return func(opts *Options) {
		opts.DryRun = d
	}
}

func WithRallyPackageFromRegistry(n, v string) OptionFunc {
	return func(opts *Options) {
		opts.PackageName = n
		opts.PackageVersion = v
	}
}

func WithRallyCorpusAtPath(c string) OptionFunc {
	return func(opts *Options) {
		opts.CorpusAtPath = c
	}
}

func WithRepositoryRoot(r *os.Root) OptionFunc {
	return func(opts *Options) {
		opts.RepositoryRoot = r
	}
}
