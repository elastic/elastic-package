// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/filter"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

const filterLongDescription = `This command would give you a list of all the packages based on the given query`

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

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func filterCommandAction(cmd *cobra.Command, args []string) error {
	filters := filter.NewFilter()

	if err := filters.Parse(cmd); err != nil {
		return fmt.Errorf("getting filter options failed: %w", err)
	}

	if err := filters.Validate(); err != nil {
		return fmt.Errorf("validating filter options failed: %w", err)
	}

	root, err := packages.MustFindIntegrationRoot()
	if err != nil {
		return fmt.Errorf("can't find integration root: %w", err)
	}

	pkgs, err := packages.ReadAllPackageManifests(root)
	if err != nil {
		return fmt.Errorf("listing packages failed: %w", err)
	}

	filtered, err := filters.ApplyTo(pkgs)
	if err != nil {
		return fmt.Errorf("filtering packages failed: %w", err)
	}

	if err := printPkgList(filtered, os.Stdout); err != nil {
		return fmt.Errorf("printing JSON failed: %w", err)
	}

	return nil
}

func printPkgList(pkgs []packages.PackageManifest, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if len(pkgs) == 0 {
		return nil
	}

	names := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		names = append(names, pkg.Name)
	}

	return enc.Encode(names)
}
