// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import "net/url"

// SkipConfig allows a test to be marked as skipped
type SkipConfig struct {
	Reason string  `config:"reason"`
	Link   url.URL `config:"url"`
}
