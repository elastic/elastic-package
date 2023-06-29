// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/elastic/elastic-package/cmd"
	"github.com/elastic/elastic-package/internal/install"
)

func main() {
	rootCmd := cmd.RootCmd()

	err := install.EnsureInstalled()
	if err != nil {
		log.Fatal(fmt.Errorf("validating installation failed: %w", err))
	}

	err = rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
