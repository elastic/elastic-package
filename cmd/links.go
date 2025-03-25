// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/cobraext"
)

const linksLongDescription = ``

func setupLinksCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "links",
		Short: "Manage linked files",
		Long:  linksLongDescription,
		RunE: func(parent *cobra.Command, args []string) error {
			return cobraext.ComposeCommandsParentContext(parent, args, parent.Commands()...)
		},
	}

	cmd.AddCommand(getLinksCheckCommand())
	cmd.AddCommand(getLinksUpdateCommand())
	cmd.AddCommand(getLinksListCommand())

	return cobraext.NewCommand(cmd, cobraext.ContextBoth)
}

func getLinksCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check for linked files changes",
		Args:  cobra.NoArgs,
		RunE:  linksCheckCommandAction,
	}
	return cmd
}

func linksCheckCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Printf("Check for linked files changes\n")
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("reading current working directory failed: %w", err)
	}

	linkedFiles, err := builder.AreLinkedFilesUpToDate(pwd)
	if err != nil {
		return fmt.Errorf("checking linked files are up-to-date failed: %w", err)
	}
	for _, f := range linkedFiles {
		if !f.UpToDate {
			cmd.Printf("%s is outdated.\n", f.Path)
		}
	}
	if len(linkedFiles) > 0 {
		return fmt.Errorf("linked files are outdated")
	}
	return nil
}

func getLinksUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update linked files checksums if needed.",
		Args:  cobra.NoArgs,
		RunE:  linksUpdateCommandAction,
	}
	return cmd
}

func linksUpdateCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Printf("Update linked files checksums if needed.\n")
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("reading current working directory failed: %w", err)
	}

	linkedFiles, err := builder.UpdateLinkedFilesChecksums(pwd)
	if err != nil {
		return fmt.Errorf("updating linked files checksums failed: %w", err)
	}

	for _, f := range linkedFiles {
		if !f.UpToDate {
			cmd.Printf("%s is outdated.\n", f.Path)
		}
	}

	return nil
}

func getLinksListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List packages linking files from this path",
		Args:  cobra.NoArgs,
		RunE:  linksListCommandAction,
	}
	return cmd
}

func linksListCommandAction(cmd *cobra.Command, args []string) error {
	cmd.Printf("List packages linking files from this path.\n")
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("reading current working directory failed: %w", err)
	}

	packages, err := builder.ListPackagesWithLinkedFilesFrom(pwd)
	if err != nil {
		return fmt.Errorf("listing linked packages failed: %w", err)
	}

	for _, p := range packages {
		cmd.Printf("%s\n", p)
	}

	return nil
}
