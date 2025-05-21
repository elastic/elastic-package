// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"

	"github.com/elastic/elastic-package/cmd"
	"github.com/elastic/elastic-package/internal/install"
)

func main() {
	err := install.EnsureInstalled()
	if err != nil {
		log.Fatalf("Validating installation failed: %v", err)
	}

	rootCmd := cmd.RootCmd()
	rootCmd.SilenceErrors = true // Silence errors so we handle them here.
	err = rootCmd.Execute()
	if errIsInterruption(err) {
		rootCmd.Println("interrupted")
		os.Exit(130)
	}
	if err != nil {
		rootCmd.PrintErrln(rootCmd.ErrPrefix(), err)
		os.Exit(1)
	}
}

func errIsInterruption(err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) && (*exitError).ProcessState.ExitCode() == 130 { // 130 -> subcommand killed by sigint
		return true
	}

	return false
}
