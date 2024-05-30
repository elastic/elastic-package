// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"log/slog"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
)

type exporter struct {
	logger *slog.Logger
	kibana *kibana.Client
}

type ExporterOption func(c *exporter)

func NewExporter(opts ...ExporterOption) *exporter {
	e := exporter{
		logger: logger.Logger,
	}

	for _, opt := range opts {
		opt(&e)
	}

	return &e
}

func WithLogger(logger *slog.Logger) ExporterOption {
	return func(e *exporter) {
		e.logger = logger
	}
}

func WithKibana(client *kibana.Client) ExporterOption {
	return func(e *exporter) {
		e.kibana = client
	}
}
