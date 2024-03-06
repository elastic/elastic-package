// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"

	"github.com/elastic/elastic-package/cmd"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
)

func main() {
	err := install.EnsureInstalled()
	if err != nil {
		log.Fatalf("Validating installation failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	stop := context.AfterFunc(ctx, func() {
		logger.Info("Signal caught!")
	})
	defer stop()

	rootCmd := cmd.RootCmd()
	rootCmd.SilenceErrors = true // Silence errors so we handle them here.
	err = rootCmd.ExecuteContext(ctx)
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
	return errors.Is(err, context.Canceled)
}
