// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stream

import (
	"os"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/profile"
)

// Options contains benchmark runner options.
type Options struct {
	ESAPI           *elasticsearch.API
	KibanaClient    *kibana.Client
	BenchName       string
	BackFill        time.Duration
	EventsPerPeriod uint64
	PeriodDuration  time.Duration
	PerformCleanup  bool
	TimestampField  string
	PackageRootPath string
	Variant         string
	Profile         *profile.Profile
	RepositoryRoot  *os.Root
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

func WithBackFill(d time.Duration) OptionFunc {
	return func(opts *Options) {
		opts.BackFill = -1 * d
	}
}

func WithEventsPerPeriod(e uint64) OptionFunc {
	return func(opts *Options) {
		opts.EventsPerPeriod = e
	}
}

func WithPeriodDuration(d time.Duration) OptionFunc {
	return func(opts *Options) {
		opts.PeriodDuration = d
	}
}

func WithPerformCleanup(p bool) OptionFunc {
	return func(opts *Options) {
		opts.PerformCleanup = p
	}
}

func WithTimestampField(t string) OptionFunc {
	return func(opts *Options) {
		opts.TimestampField = t
	}
}

func WithRepositoryRoot(r *os.Root) OptionFunc {
	return func(opts *Options) {
		opts.RepositoryRoot = r
	}
}
