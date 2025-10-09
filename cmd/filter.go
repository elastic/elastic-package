// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/elastic/elastic-package/internal/cobraext"
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
	cmd.Println("Filter the package")

	opts, err := fromFlags(cmd)
	if err != nil {
		return fmt.Errorf("getting filter options failed: %w", err)
	}

	if err := opts.Validate(); err != nil {
		return fmt.Errorf("validating filter options failed: %w", err)
	}

	fmt.Println(opts.String())

	return nil
}

type filterOptions struct {
	input         string
	codeOwner     string
	kibanaVersion string
	categories    string
}

func fromFlags(cmd *cobra.Command) (*filterOptions, error) {
	input, err := cmd.Flags().GetString(cobraext.FilterInputFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterInputFlagName)
	}

	codeOwner, err := cmd.Flags().GetString(cobraext.FilterCodeOwnerFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterCodeOwnerFlagName)
	}

	kibanaVersion, err := cmd.Flags().GetString(cobraext.FilterKibanaVersionFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterKibanaVersionFlagName)
	}

	categories, err := cmd.Flags().GetString(cobraext.FilterCategoriesFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterCategoriesFlagName)
	}

	return &filterOptions{
		input:         input,
		codeOwner:     codeOwner,
		kibanaVersion: kibanaVersion,
		categories:    categories,
	}, nil
}

func (o *filterOptions) String() string {
	return fmt.Sprintf("input: %s, codeOwner: %s, kibanaVersion: %s, categories: %s", o.input, o.codeOwner, o.kibanaVersion, o.categories)
}

func (o *filterOptions) Validate() error {
	if o.input == "" && o.codeOwner == "" && o.kibanaVersion == "" && o.categories == "" {
		return fmt.Errorf("at least one flag must be provided")
	}

	if _, err := semver.NewConstraint(o.kibanaVersion); err != nil {
		return fmt.Errorf("invalid kibana version: %w", err)
	}
	return nil
}
