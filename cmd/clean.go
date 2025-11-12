// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cleanup"
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/install"
)

const cleanLongDescription = `Use this command to clean resources used for building the package.

The command will remove built package files (in build/), files needed for managing the development stack (in ~/.elastic-package/stack/development) and stack service logs (in ~/.elastic-package/tmp/service_logs and ~/.elastic-package/profiles/<profile>/service_logs/).`

func setupCleanCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean used resources",
		Long:  cleanLongDescription,
		Args:  cobra.NoArgs,
		RunE:  cleanCommandAction,
	}
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func cleanCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Clean used resources")

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	cwd, err := cobraext.Getwd(cmd)
	if err != nil {
		return err
	}

	target, err := cleanup.Build(cwd)
	if err != nil {
		return fmt.Errorf("can't clean build resources: %w", err)
	}

	if target != "" {
		cmd.Printf("Build resources removed: %s\n", target)
	}

	target, err = cleanup.Stack(cwd)
	if err != nil {
		return fmt.Errorf("can't clean the development stack: %w", err)
	}
	if target != "" {
		cmd.Printf("Package removed from the development stack: %s\n", target)
	}

	target, err = cleanup.ServiceLogs()
	if err != nil {
		return fmt.Errorf("can't clean temporary service logs: %w", err)
	}
	if target != "" {
		cmd.Printf("Temporary service logs removed: %s\n", target)
	}

	target, err = cleanup.ServiceLogsIndependentAgents(profile, cwd)
	if err != nil {
		return fmt.Errorf("can't clean temporary service logs: %w", err)
	}
	if target != "" {
		cmd.Printf("Temporary service logs (independent Elastic Agents) removed: %s\n", target)
	}

	cmd.Println("Done")
	return nil
}
