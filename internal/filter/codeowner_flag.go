// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/tui"
)

type CodeOwnerFlag struct {
	FilterFlagBase
	values map[string]struct{}
}

func (f *CodeOwnerFlag) Parse(cmd *cobra.Command) error {
	codeOwners, err := cmd.Flags().GetString(cobraext.FilterCodeOwnerFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterCodeOwnerFlagName)
	}
	if codeOwners == "" {
		return nil
	}

	f.values = splitAndTrim(codeOwners, ",")
	f.isApplied = true
	return nil
}

func (f *CodeOwnerFlag) Validate() error {
	validator := tui.Validator{Cwd: "."}

	if f.values != nil {
		for value := range f.values {
			if err := validator.GithubOwner(value); err != nil {
				return fmt.Errorf("invalid code owner: %s: %w", value, err)
			}
		}
	}

	return nil
}

func (f *CodeOwnerFlag) Matches(dirName string, manifest *packages.PackageManifest) bool {
	return hasAnyMatch(f.values, []string{manifest.Owner.Github})
}

func (f *CodeOwnerFlag) ApplyTo(pkgs []packages.PackageDirNameAndManifest) ([]packages.PackageDirNameAndManifest, error) {
	filtered := make([]packages.PackageDirNameAndManifest, 0, len(pkgs))
	for _, pkg := range pkgs {
		if f.Matches(pkg.DirName, pkg.Manifest) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, nil
}

func initCodeOwnerFlag() *CodeOwnerFlag {
	return &CodeOwnerFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterCodeOwnerFlagName,
			description:  cobraext.FilterCodeOwnerFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
