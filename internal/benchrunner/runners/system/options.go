// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/profile"
)

// Options contains benchmark runner options.
type Options struct {
	WorkDir         string
	ESAPI           *elasticsearch.API
	KibanaClient    *kibana.Client
	DeferCleanup    time.Duration
	MetricsInterval time.Duration
	ReindexData     bool
	ESMetricsAPI    *elasticsearch.API
	BenchPath       string
	BenchName       string
	PackageRootPath string
	Variant         string
	Profile         *profile.Profile
}

type OptionFunc func(*Options)

func NewOptions(fns ...OptionFunc) Options {
	var opts Options
	for _, fn := range fns {
		fn(&opts)
	}
	return opts
}

func WithWorkDir(workDir string) OptionFunc {
	return func(opts *Options) {
		opts.WorkDir = workDir
	}
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

func WithBenchmarkPath(path string) OptionFunc {
	return func(opts *Options) {
		opts.BenchPath = path
	}
}

func WithBenchmarkName(name string) OptionFunc {
	return func(opts *Options) {
		opts.BenchName = name
	}
}

func WithDeferCleanup(d time.Duration) OptionFunc {
	return func(opts *Options) {
		opts.DeferCleanup = d
	}
}

func WithMetricsInterval(d time.Duration) OptionFunc {
	return func(opts *Options) {
		opts.MetricsInterval = d
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
