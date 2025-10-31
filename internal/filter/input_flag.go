// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
)

type InputFlag struct {
	FilterFlagBase

	// flag specific fields
	values map[string]struct{}
}

func (f *InputFlag) Parse(cmd *cobra.Command) error {
	input, err := cmd.Flags().GetString(cobraext.FilterInputFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterInputFlagName)
	}
	if input == "" {
		return nil
	}

	f.values = splitAndTrim(input, ",")
	f.isApplied = true
	return nil
}

func (f *InputFlag) Validate() error {
	return nil
}

func (f *InputFlag) Matches(dirName string, manifest *packages.PackageManifest) bool {
	if f.values != nil {
		inputs := extractInputs(manifest)
		if !hasAnyMatch(f.values, inputs) {
			return false
		}
	}
	return true
}

func (f *InputFlag) ApplyTo(pkgs []packages.PackageDirNameAndManifest) ([]packages.PackageDirNameAndManifest, error) {
	filtered := make([]packages.PackageDirNameAndManifest, 0, len(pkgs))

	for _, pkg := range pkgs {
		if f.Matches(pkg.DirName, pkg.Manifest) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, nil
}

func initInputFlag() *InputFlag {
	return &InputFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterInputFlagName,
			description:  cobraext.FilterInputFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
