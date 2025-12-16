// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"fmt"

	"github.com/gobwas/glob"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
)

type PackageDirNameFlag struct {
	FilterFlagBase

	patterns []glob.Glob
}

func (f *PackageDirNameFlag) Parse(cmd *cobra.Command) error {
	packageDirNamePatterns, err := cmd.Flags().GetString(cobraext.FilterPackageDirNameFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterPackageDirNameFlagName)
	}

	patterns := splitAndTrim(packageDirNamePatterns, ",")
	for _, patternString := range patterns {
		pattern, err := glob.Compile(patternString)
		if err != nil {
			return fmt.Errorf("invalid package dir name pattern: %s: %w", patternString, err)
		}
		f.patterns = append(f.patterns, pattern)
	}

	if len(f.patterns) > 0 {
		f.isApplied = true
	}

	return nil
}

func (f *PackageDirNameFlag) Validate() error {
	return nil
}

func (f *PackageDirNameFlag) Matches(dirName string, manifest *packages.PackageManifest) bool {
	for _, pattern := range f.patterns {
		if pattern.Match(dirName) {
			return true
		}
	}
	return false
}

func (f *PackageDirNameFlag) ApplyTo(pkgs []packages.PackageDirNameAndManifest) ([]packages.PackageDirNameAndManifest, error) {
	filtered := make([]packages.PackageDirNameAndManifest, 0, len(pkgs))
	for _, pkg := range pkgs {
		if f.Matches(pkg.DirName, pkg.Manifest) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, nil
}

func initPackageDirNameFlag() *PackageDirNameFlag {
	return &PackageDirNameFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterPackageDirNameFlagName,
			description:  cobraext.FilterPackageDirNameFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
