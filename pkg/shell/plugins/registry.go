// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"github.com/elastic/elastic-package/pkg/shell"
)

func init() {
	Registry.commands = append(
		[]shell.Command{},
		changelogCmd{},
		writefileCmd{},
		whereCmd{},
		initdbCmd{},
	)
}

type ctxKey string

const (
	ctxKeyPackages ctxKey = "Shell.Packages"
	ctxKeyDB       ctxKey = "Shell.DB"
)

var Registry = registry{}

var _ shell.Plugin = registry{}

type registry struct {
	commands []shell.Command
}

func (r registry) Commands() []shell.Command {
	return r.commands
}
