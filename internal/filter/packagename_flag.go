package filter

import (
	"fmt"
	"regexp"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type PackageNameFlag struct {
	FilterFlagBase

	patterns []*regexp.Regexp
}

func (f *PackageNameFlag) Parse(cmd *cobra.Command) error {
	packageNamePatterns, err := cmd.Flags().GetString(cobraext.FilterPackagesFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterPackagesFlagName)
	}

	patternStrings := splitAndTrim(packageNamePatterns, ",")
	for patternString := range patternStrings {
		regex, err := regexp.Compile(patternString)
		if err != nil {
			return fmt.Errorf("invalid package name pattern: %s: %w", patternString, err)
		}
		f.patterns = append(f.patterns, regex)
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
		if pattern.MatchString(pkgDirName) {
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
