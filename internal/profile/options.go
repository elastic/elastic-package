// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

// Options defines available stack management options
type Options struct {
	PackagePath       string
	Name              string
	FromProfile       string
	OverwriteExisting bool
}
