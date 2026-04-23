// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tliron/commonlog"
	_ "github.com/tliron/commonlog/simple"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/lsp"
)

const lspLongDescription = `Start a Language Server Protocol (LSP) server for Elastic integration packages.

The LSP server communicates over stdin/stdout and provides real-time validation
diagnostics for integration packages opened in supported editors.`

func setupLSPCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "Start the LSP server",
		Long:  lspLongDescription,
		Args:  cobra.NoArgs,
		RunE:  lspCommandAction,
		// Override the parent's PersistentPreRunE to prevent version check
		// messages and install output from corrupting the JSON-RPC stdio stream.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().String("log-file", "", "Path to log file for debug output")

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func lspCommandAction(cmd *cobra.Command, args []string) error {
	logFile, _ := cmd.Flags().GetString("log-file")
	if logFile != "" {
		commonlog.Configure(2, &logFile)
	}

	s := lsp.NewServer()
	return s.RunStdio()
}
