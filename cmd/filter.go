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

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/filter"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
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

	// add the output package name flag to the command
	cmd.Flags().BoolP(cobraext.FilterOutputPackageNameFlagName, cobraext.FilterOutputPackageNameFlagShorthand, false, cobraext.FilterOutputPackageNameFlagDescription)

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

	if err = printPkgList(filtered, printPackageName, os.Stdout); err != nil {
		return fmt.Errorf("printing JSON failed: %w", err)
	}

	return nil
}

func filterPackage(cmd *cobra.Command) (map[string]packages.PackageManifest, error) {
	filters := filter.NewFilterRegistry()

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

func printPkgList(pkgs map[string]packages.PackageManifest, printPackageName bool, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if len(pkgs) == 0 {
		return nil
	}

	names := make([]string, 0, len(pkgs))
	if printPackageName {
		for _, pkgManifest := range pkgs {
			names = append(names, pkgManifest.Name)
		}
	} else {
		for pkgDirName := range pkgs {
			names = append(names, pkgDirName)
		}
	}

	slices.Sort(names)
	return enc.Encode(names)
}
