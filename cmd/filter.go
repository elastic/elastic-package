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
	"strings"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/tui"
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

	cmd.Flags().StringP(cobraext.FilterInputFlagName, "", "", cobraext.FilterInputFlagDescription)
	cmd.Flags().StringP(cobraext.FilterCodeOwnerFlagName, "", "", cobraext.FilterCodeOwnerFlagDescription)
	cmd.Flags().StringP(cobraext.FilterKibanaVersionFlagName, "", "", cobraext.FilterKibanaVersionFlagDescription)
	cmd.Flags().StringP(cobraext.FilterCategoriesFlagName, "", "", cobraext.FilterCategoriesFlagDescription)
	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func filterCommandAction(cmd *cobra.Command, args []string) error {
	opts, err := fromFlags(cmd)
	if err != nil {
		return fmt.Errorf("getting filter options failed: %w", err)
	}

	if err := opts.Validate(); err != nil {
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

	filtered, err := opts.Filter(pkgs)
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

	names := []string{}
	for _, pkg := range pkgs {
		names = append(names, pkg.Name)
	}

	return enc.Encode(names)
}

type filterOptions struct {
	inputs         []string
	codeOwners     []string
	kibanaVersions []string
	categories     []string
}

func fromFlags(cmd *cobra.Command) (*filterOptions, error) {
	opts := &filterOptions{}

	input, err := cmd.Flags().GetString(cobraext.FilterInputFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterInputFlagName)
	}
	if input != "" {
		opts.inputs = strings.Split(input, ",")
	}

	codeOwner, err := cmd.Flags().GetString(cobraext.FilterCodeOwnerFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterCodeOwnerFlagName)
	}
	if codeOwner != "" {
		opts.codeOwners = strings.Split(codeOwner, ",")
	}

	kibanaVersion, err := cmd.Flags().GetString(cobraext.FilterKibanaVersionFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterKibanaVersionFlagName)
	}
	if kibanaVersion != "" {
		opts.kibanaVersions = strings.Split(kibanaVersion, ",")
	}

	categories, err := cmd.Flags().GetString(cobraext.FilterCategoriesFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterCategoriesFlagName)
	}
	if categories != "" {
		opts.categories = strings.Split(categories, ",")
	}

	return opts, nil
}

func (o *filterOptions) String() string {
	return fmt.Sprintf("input: %s, codeOwner: %s, kibanaVersion: %s, categories: %s", o.inputs, o.codeOwners, o.kibanaVersions, o.categories)
}

func (o *filterOptions) Validate() error {
	validator := tui.Validator{Cwd: "."}

	if len(o.inputs) == 0 && len(o.codeOwners) == 0 && len(o.kibanaVersions) == 0 && len(o.categories) == 0 {
		return fmt.Errorf("at least one flag must be provided")
	}

	if len(o.kibanaVersions) > 0 {
		for _, version := range o.kibanaVersions {
			if err := validator.Constraint(version); err != nil {
				return fmt.Errorf("invalid kibana version: %w", err)
			}
		}
	}

	if len(o.codeOwners) > 0 {
		for _, owner := range o.codeOwners {
			if err := validator.GithubOwner(owner); err != nil {
				return fmt.Errorf("invalid code owner: %w", err)
			}
		}
	}

	return nil
}

func (o *filterOptions) Check(pkg packages.PackageManifest) bool {
	codeOwner := pkg.Owner.Github
	// kibanaVersion := pkg.Conditions.Kibana.Version
	categories := pkg.Categories
	inputs := []string{}
	for _, policyTemplate := range pkg.PolicyTemplates {
		if policyTemplate.Input != "" {
			inputs = append(inputs, policyTemplate.Input)
		}

		for _, input := range policyTemplate.Inputs {
			inputs = append(inputs, input.Type)
		}
	}

	if len(o.inputs) > 0 {
		exists := false
		for _, input := range inputs {
			if slices.Contains(o.inputs, input) {
				exists = true
				break
			}
		}

		if !exists {
			return false
		}
	}

	if len(o.codeOwners) > 0 {
		if !slices.Contains(o.codeOwners, codeOwner) {
			return false
		}
	}

	if len(o.categories) > 0 {
		exists := false
		for _, category := range categories {
			if slices.Contains(o.categories, category) {
				exists = true
				break
			}
		}

		if !exists {
			return false
		}
	}

	// TODO: check kibana version
	if len(o.kibanaVersions) > 0 {
		panic("not implemented")
	}

	return true
}

func (o *filterOptions) Filter(pkgs []packages.PackageManifest) ([]packages.PackageManifest, error) {
	var filteredPackages []packages.PackageManifest

	for _, pkg := range pkgs {
		if !o.Check(pkg) {
			continue
		}
		filteredPackages = append(filteredPackages, pkg)
	}

	return filteredPackages, nil
}
