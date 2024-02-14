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
)

func main() {
	rootCmd := cmd.RootCmd()

	err := install.EnsureInstalled()
	if err != nil {
		log.Fatalf("Validating installation failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err = rootCmd.ExecuteContext(ctx)
	if errIsInterruption(err) {
		os.Exit(130)
	}
	if err != nil {
		os.Exit(1)
	}
}

func errIsInterruption(err error) bool {
	return errors.Is(err, context.Canceled)
}
