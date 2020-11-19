// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/docs"
)

func setupBuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the package",
		Long:  "Use build command to build the package.",
		RunE:  buildCommandAction,
	}
	return cmd
}

func buildCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Println("Build the package")

	target, err := docs.UpdateReadme()
	if err != nil {
		return errors.Wrapf(err, "updating %s file failed", docs.ReadmeFile)
	}
	if target != "" {
		cmd.Printf("%s file rendered: %s\n", docs.ReadmeFile, target)
	}

	target, err = builder.BuildPackage()
	if err != nil {
		return errors.Wrap(err, "building package failed")
	}
	cmd.Printf("Package built: %s\n", target)

	cmd.Println("Done")
	return nil
}
