package filter

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type SpecVersionFlag struct {
	FilterFlag

	// package spec version constraint
	constraints *semver.Constraints
}

func (f *SpecVersionFlag) Parse(cmd *cobra.Command) error {

	formatVersion, err := cmd.Flags().GetString(cobraext.FilterSpecVersionFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterSpecVersionFlagName)
	}

	constraint, err := semver.NewConstraint(formatVersion)
	if err != nil {
		return fmt.Errorf("invalid format version: %s: %w", formatVersion, err)
	}

	f.constraints = constraint

	return nil
}

func (f *SpecVersionFlag) Validate() error {
	// no validation needed for this flag
	// checks are done in Parse method
	return nil
}

func (f *SpecVersionFlag) Matches(pkg packages.PackageManifest) bool {
	// assuming the package spec version in manifest.yml is a valid semver
	pkgVersion, _ := semver.NewVersion(pkg.SpecVersion)

	return f.constraints.Check(pkgVersion)
}

func (f *SpecVersionFlag) ApplyTo(pkgs []packages.PackageManifest) (filtered []packages.PackageManifest, err error) {
	for _, pkg := range pkgs {
		if f.Matches(pkg) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, nil
}

func initSpecVersionFlag() *SpecVersionFlag {
	return &SpecVersionFlag{
		FilterFlag: FilterFlag{
			name:         cobraext.FilterSpecVersionFlagName,
			description:  cobraext.FilterSpecVersionFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
