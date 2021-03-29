// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

// CommandInfos contains all elastic-package commands' information.
var CommandInfos = map[string]CommandInfo{}

// CommandInfo encapsulates information about an elastic-package command.
type CommandInfo struct {
	// Short description of command
	Short string

	// Long description of command
	Long string

	// Context of command: global or package
	Context string
}

// LongCLI generates a command's long description for displaying in
// the elastic-package CLI help text.
func (c CommandInfo) LongCLI() string {
	return c.Long + "\n\n" + "Context: " + c.Context
}
