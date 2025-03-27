// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

import (
	"fmt"

	"github.com/spf13/cobra"
)

// CommandContext is the context in which an elastic-package command runs.
type CommandContext string

const (
	// ContextGlobal means the command runs in a global context, agnostic of any
	// specific package.
	ContextGlobal CommandContext = "global"

	// ContextPackage means the command runs in the contexts of a specific package.
	ContextPackage CommandContext = "package"
)

// Command wraps a cobra.Command and adds some additional information relevant
// to elastic-package commands.
type Command struct {
	*cobra.Command

	longDesc string

	// Context of command: global or package
	ctxt CommandContext
}

// NewCommand creates a new Command
func NewCommand(cmd *cobra.Command, context CommandContext) *Command {
	c := Command{
		Command: cmd,
		ctxt:    context,
	}

	c.longDesc = cmd.Long
	cmd.Long = fmt.Sprintf("%s\n\nContext: %s\n", c.longDesc, c.ctxt)

	return &c
}

// Name returns the name of the elastic-package command.
func (c *Command) Name() string {
	return c.Command.Use
}

// Short returns a short description for the elastic-package command.
func (c *Command) Short() string {
	return c.Command.Short
}

// Long returns a long description for the elastic-package command.
func (c *Command) Long() string {
	return c.longDesc
}

// Context returns the context for the elastic-package command.
func (c *Command) Context() CommandContext {
	return c.ctxt
}
