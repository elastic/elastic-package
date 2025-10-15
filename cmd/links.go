// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/files"
)

const (
	linksLongDescription       = `Use this command to manage linked files in the repository.`
	linksCheckLongDescription  = `Use this command to check if linked files references inside the current directory are up to date.`
	linksUpdateLongDescription = `Use this command to update all linked files references inside the current directory.`
	linksListLongDescription   = `Use this command to list all packages that have linked file references that include the current directory.`
)

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

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func getLinksCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check for linked files changes",
		Long:  linksCheckLongDescription,
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
	// Find the repository root to create the links filesystem reference tied to the repository root
	repositoryRoot, err := files.FindRepositoryRoot()
	if err != nil {
		return fmt.Errorf("finding repository root: %w", err)
	}
	defer repositoryRoot.Close()

	linksFS, err := files.CreateLinksFSFromPath(repositoryRoot, pwd)
	if err != nil {
		return fmt.Errorf("creating links filesystem failed: %w", err)
	}

	linkedFiles, err := linksFS.CheckLinkedFiles()
	if err != nil {
		return fmt.Errorf("checking linked files are up-to-date failed: %w", err)
	}
	for _, f := range linkedFiles {
		if !f.UpToDate {
			cmd.Printf("%s is outdated.\n", f.LinkFilePath)
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
		Long:  linksUpdateLongDescription,
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

	// Find the repository root to create the links filesystem reference tied to the repository root
	repositoryRoot, err := files.FindRepositoryRoot()
	if err != nil {
		return fmt.Errorf("finding repository root: %w", err)
	}
	defer repositoryRoot.Close()

	linksFS, err := files.CreateLinksFSFromPath(repositoryRoot, pwd)
	if err != nil {
		return fmt.Errorf("creating links filesystem failed: %w", err)
	}

	updatedLinks, err := linksFS.UpdateLinkedFiles()
	if err != nil {
		return fmt.Errorf("updating linked files checksums failed: %w", err)
	}

	for _, l := range updatedLinks {
		cmd.Printf("%s was updated.\n", l.LinkFilePath)
	}

	return nil
}

func getLinksListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List packages linking files from this path",
		Long:  linksListLongDescription,
		Args:  cobra.NoArgs,
		RunE:  linksListCommandAction,
	}
	cmd.Flags().BoolP(cobraext.PackagesFlagName, "", false, cobraext.PackagesFlagDescription)
	return cmd
}

func linksListCommandAction(cmd *cobra.Command, args []string) error {
	onlyPackages, err := cmd.Flags().GetBool(cobraext.PackagesFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackagesFlagName)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("reading current working directory failed: %w", err)
	}

	// Find the repository root to create the links filesystem reference tied to the repository root
	repositoryRoot, err := files.FindRepositoryRoot()
	if err != nil {
		return fmt.Errorf("finding repository root: %w", err)
	}
	defer repositoryRoot.Close()

	linksFS, err := files.CreateLinksFSFromPath(repositoryRoot, pwd)
	if err != nil {
		return fmt.Errorf("creating links filesystem failed: %w", err)
	}

	byPackage, err := linksFS.ListLinkedFilesByPackage()
	if err != nil {
		return fmt.Errorf("listing linked packages failed: %w", err)
	}

	for _, pkg := range byPackage {
		if onlyPackages {
			cmd.Println(pkg.PackageName)
			continue
		}
		for _, link := range pkg.Links {
			cmd.Println(link)
		}
	}

	return nil
}
