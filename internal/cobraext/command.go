// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

import (
	"fmt"

	"github.com/spf13/cobra"
)

type CommandContext string

const (
	ContextGlobal  CommandContext = "global"
	ContextPackage CommandContext = "package"
)

type Command struct {
	*cobra.Command

	longDesc string

	// Context of command: global or package
	ctxt CommandContext
}

func NewCommand(cmd *cobra.Command, context CommandContext) *Command {
	c := Command{
		Command: cmd,
		ctxt:    context,
	}

	c.longDesc = cmd.Long
	cmd.Long = fmt.Sprintf("%s\n\nContext: %s\n", c.longDesc, c.ctxt)

	return &c
}

func (c *Command) Name() string {
	return c.Command.Use
}

func (c *Command) Short() string {
	return c.Command.Short
}

func (c *Command) Long() string {
	return c.longDesc
}

func (c *Command) Context() CommandContext {
	return c.ctxt
}
