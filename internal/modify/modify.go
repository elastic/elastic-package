// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package modify

import (
	"github.com/spf13/pflag"

	"github.com/elastic/elastic-package/internal/fleetpkg"
)

type Modifier struct {
	Name  string
	Doc   string
	Flags pflag.FlagSet
	Run   func(pkg *fleetpkg.Package) error
}
