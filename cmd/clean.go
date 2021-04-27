// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cleanup"
	"github.com/elastic/elastic-package/internal/cobraext"
)

const cleanLongDescription = `Use this command to clean resources used for building the package.

The command will remove built package files (in build/), files needed for managing the development stack (in ~/.elastic-package/stack/development) and stack service logs (in ~/.elastic-package/tmp/service_logs).`

func setupCleanCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean used resources",
		Long:  cleanLongDescription,
		RunE:  cleanCommandAction,
	}

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func cleanCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Clean used resources")

	target, err := cleanup.Build()
	if err != nil {
		return errors.Wrap(err, "can't clean build resources")
	}

	if target != "" {
		cmd.Printf("Build resources removed: %s\n", target)
	}

	target, err = cleanup.Stack()
	if err != nil {
		return errors.Wrap(err, "can't clean the development stack")
	}
	if target != "" {
		cmd.Printf("Package removed from the development stack: %s\n", target)
	}

	target, err = cleanup.ServiceLogs()
	if err != nil {
		return errors.Wrap(err, "can't clean temporary service logs")
	}
	if target != "" {
		cmd.Printf("Temporary service logs removed: %s\n", target)
	}

	cmd.Println("Done")
	return nil
}
