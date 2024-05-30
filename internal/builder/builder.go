// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"log/slog"

	"github.com/elastic/elastic-package/internal/logger"
)

type BuildOptions struct {
	PackageRoot string

	CreateZip      bool
	SignPackage    bool
	SkipValidation bool

	Logger *slog.Logger
}

type packageBuilder struct {
	packageRoot    string
	skipValidation bool
	createZip      bool
	signPackage    bool
	logger         *slog.Logger
}

func NewPackageBuilder(options BuildOptions) *packageBuilder {
	b := packageBuilder{
		logger:         logger.Logger,
		packageRoot:    options.PackageRoot,
		createZip:      options.CreateZip,
		skipValidation: options.SkipValidation,
		signPackage:    options.SignPackage,
	}
	if options.Logger != nil {
		b.logger = options.Logger
	}

	b.logger = b.logger.With(slog.String("package.path", options.PackageRoot))
	return &b
}
