// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/cmd"
	"github.com/elastic/elastic-package/internal/install"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	rootCmd := cmd.RootCmd()

	err := install.EnsureInstalled()
	if err != nil {
		log.Fatal(errors.Wrap(err, "validating installation failed"))
	}

	err = rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
