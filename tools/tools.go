// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// +build tools

package tools

// Add dependencies on tools.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
import (
	_ "github.com/elastic/go-licenser"
	_ "golang.org/x/lint/golint"
	_ "golang.org/x/tools/cmd/goimports"
)
