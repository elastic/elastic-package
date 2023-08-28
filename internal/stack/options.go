// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"github.com/elastic/elastic-package/internal/profile"
)

// Options defines available image booting options.
type Options struct {
	DaemonMode   bool
	StackVersion string

	Services []string

	Profile *profile.Profile
	Printer Printer

	// User parameters can be passed from flags to the stack provider.
	// The stack provider has the responsibility of validating and storing
	// them if needed.
	UserParameters map[string]string
}
