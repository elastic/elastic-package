// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	cshell "github.com/brianstrauch/cobra-shell"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/pkg/shell"
)

func setupShellCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:          "shell",
		Hidden:       true,
		SilenceUsage: true,
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.CompletionOptions.HiddenDefaultCmd = true

	shell.AttachCommands(cmd)

	shellCmd := cshell.New(cmd, nil)

	return cobraext.NewCommand(shellCmd, cobraext.ContextGlobal)
}
