package filter

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/gobwas/glob"
	"github.com/spf13/cobra"
)

type PackageNameFlag struct {
	FilterFlagBase

	patterns []glob.Glob
}

func (f *PackageNameFlag) Parse(cmd *cobra.Command) error {
	packageNamePatterns, err := cmd.Flags().GetString(cobraext.FilterPackagesFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterPackagesFlagName)
	}

	patterns := splitAndTrim(packageNamePatterns, ",")
	for patternString := range patterns {
		pattern, err := glob.Compile(patternString)
		if err != nil {
			return fmt.Errorf("invalid package name pattern: %s: %w", patternString, err)
		}
		f.patterns = append(f.patterns, pattern)
	}

	if len(f.patterns) > 0 {
		f.isApplied = true
	}

	return nil
}

func (f *PackageNameFlag) Validate() error {
	return nil
}

func (f *PackageNameFlag) Matches(pkgDirName string, pkgManifest packages.PackageManifest) bool {
	for _, pattern := range f.patterns {
		if pattern.Match(pkgDirName) {
			return true
		}
	}
	return false
}

func (f *PackageNameFlag) ApplyTo(pkgs map[string]packages.PackageManifest) (map[string]packages.PackageManifest, error) {
	filtered := make(map[string]packages.PackageManifest, len(pkgs))
	for pkgDirName, pkgManifest := range pkgs {
		if f.Matches(pkgDirName, pkgManifest) {
			filtered[pkgDirName] = pkgManifest
		}
	}
	return filtered, nil
}

func initPackageNameFlag() *PackageNameFlag {
	return &PackageNameFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterPackagesFlagName,
			description:  cobraext.FilterPackagesFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
