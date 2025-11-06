// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/filter"
	"github.com/elastic/elastic-package/internal/packages"
)

const filterLongDescription = `This command gives you a list of all packages based on the given query`

func setupFilterCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "filter [flags]",
		Short: "filter integrations based on given flags",
		Long:  filterLongDescription,
		Args:  cobra.NoArgs,
		RunE:  filterCommandAction,
	}

	// add filter flags to the command (input, code owner, kibana version, categories)
	filter.SetFilterFlags(cmd)

	// add the output package name and absolute path flags to the command
	cmd.Flags().BoolP(cobraext.FilterOutputPackageNameFlagName, "", false, cobraext.FilterOutputPackageNameFlagDescription)
	cmd.Flags().BoolP(cobraext.FilterOutputAbsolutePathFlagName, "", false, cobraext.FilterOutputAbsolutePathFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func filterCommandAction(cmd *cobra.Command, args []string) error {
	filtered, err := filterPackage(cmd)
	if err != nil {
		return fmt.Errorf("filtering packages failed: %w", err)
	}

	printPackageName, err := cmd.Flags().GetBool(cobraext.FilterOutputPackageNameFlagName)
	if err != nil {
		return fmt.Errorf("getting output package name flag failed: %w", err)
	}

	outputAbsolutePath, err := cmd.Flags().GetBool("output-absolute-path")
	if err != nil {
		return fmt.Errorf("getting output absolute path flag failed: %w", err)
	}

	if err = printPkgList(filtered, printPackageName, outputAbsolutePath, os.Stdout); err != nil {
		return fmt.Errorf("printing JSON failed: %w", err)
	}

	return nil
}

func filterPackage(cmd *cobra.Command) ([]packages.PackageDirNameAndManifest, error) {
	depth, err := cmd.Flags().GetInt(cobraext.FilterDepthFlagName)
	if err != nil {
		return nil, fmt.Errorf("getting depth flag failed: %w", err)
	}

	excludeDirs, err := cmd.Flags().GetString(cobraext.FilterExcludeDirFlagName)
	if err != nil {
		return nil, fmt.Errorf("getting exclude-dir flag failed: %w", err)
	}

	filters := filter.NewFilterRegistry(depth, excludeDirs)

	if err := filters.Parse(cmd); err != nil {
		return nil, fmt.Errorf("parsing filter options failed: %w", err)
	}

	if err := filters.Validate(); err != nil {
		return nil, fmt.Errorf("validating filter options failed: %w", err)
	}

	filtered, errors := filters.Execute()
	if errors != nil {
		return nil, fmt.Errorf("filtering packages failed: %s", errors.Error())
	}

	return filtered, nil
}

func printPkgList(pkgs []packages.PackageDirNameAndManifest, printPackageName bool, outputAbsolutePath bool, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if len(pkgs) == 0 {
		return nil
	}

	names := make([]string, 0, len(pkgs))
	if printPackageName {
		for _, pkg := range pkgs {
			names = append(names, pkg.Manifest.Name)
		}
	} else if outputAbsolutePath {
		for _, pkg := range pkgs {
			names = append(names, pkg.Path)
		}
	} else {
		for _, pkg := range pkgs {
			names = append(names, pkg.DirName)
		}
	}

	slices.Sort(names)
	return enc.Encode(names)
}
