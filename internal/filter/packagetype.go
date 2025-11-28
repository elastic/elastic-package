// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
)

type PackageTypeFlag struct {
	FilterFlagBase

	// flag specific fields
	values []string
}

func (f *PackageTypeFlag) Parse(cmd *cobra.Command) error {
	packageTypes, err := cmd.Flags().GetString(cobraext.FilterPackageTypeFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterPackageTypeFlagName)
	}
	if packageTypes == "" {
		return nil
	}
	f.values = splitAndTrim(packageTypes, ",")
	f.isApplied = true
	return nil
}

func (f *PackageTypeFlag) Validate() error {
	return nil
}

func (f *PackageTypeFlag) Matches(dirName string, manifest *packages.PackageManifest) bool {
	return hasAnyMatch(f.values, []string{manifest.Type})
}

func (f *PackageTypeFlag) ApplyTo(pkgs []packages.PackageDirNameAndManifest) ([]packages.PackageDirNameAndManifest, error) {
	filtered := make([]packages.PackageDirNameAndManifest, 0, len(pkgs))
	for _, pkg := range pkgs {
		if f.Matches(pkg.DirName, pkg.Manifest) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, nil
}

func initPackageTypeFlag() *PackageTypeFlag {
	return &PackageTypeFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterPackageTypeFlagName,
			description:  cobraext.FilterPackageTypeFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
