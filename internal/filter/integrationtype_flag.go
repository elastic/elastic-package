// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
)

type IntegrationTypeFlag struct {
	FilterFlagBase

	// flag specific fields
	values map[string]struct{}
}

func (f *IntegrationTypeFlag) Parse(cmd *cobra.Command) error {
	integrationTypes, err := cmd.Flags().GetString(cobraext.FilterIntegrationTypeFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterIntegrationTypeFlagName)
	}
	if integrationTypes == "" {
		return nil
	}
	f.values = splitAndTrim(integrationTypes, ",")
	f.isApplied = true
	return nil
}

func (f *IntegrationTypeFlag) Validate() error {
	return nil
}

func (f *IntegrationTypeFlag) Matches(dirName string, manifest *packages.PackageManifest) bool {
	if f.values != nil {
		if !hasAnyMatch(f.values, []string{manifest.Type}) {
			return false
		}
	}
	return true
}

func (f *IntegrationTypeFlag) ApplyTo(pkgs []packages.PackageDirNameAndManifest) ([]packages.PackageDirNameAndManifest, error) {
	filtered := make([]packages.PackageDirNameAndManifest, 0, len(pkgs))
	for _, pkg := range pkgs {
		if f.Matches(pkg.DirName, pkg.Manifest) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, nil
}

func initIntegrationTypeFlag() *IntegrationTypeFlag {
	return &IntegrationTypeFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterIntegrationTypeFlagName,
			description:  cobraext.FilterIntegrationTypeFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
