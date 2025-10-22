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
	specVersion, err := cmd.Flags().GetString(cobraext.FilterSpecVersionFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterSpecVersionFlagName)
	}
	if specVersion == "" {
		return nil
	}

	f.constraints, err = semver.NewConstraint(specVersion)
	if err != nil {
		return fmt.Errorf("invalid spec version: %s: %w", specVersion, err)
	}

	f.isApplied = true
	return nil
}

func (f *SpecVersionFlag) Validate() error {
	// no validation needed for this flag
	// checks are done in Parse method
	return nil
}

func (f *SpecVersionFlag) Matches(pkgName string, pkg packages.PackageManifest) bool {
	// assuming the package spec version in manifest.yml is a valid semver
	pkgVersion, _ := semver.NewVersion(pkg.SpecVersion)

	return f.constraints.Check(pkgVersion)
}

func (f *SpecVersionFlag) ApplyTo(pkgs map[string]packages.PackageManifest) (filtered map[string]packages.PackageManifest, err error) {
	filtered = make(map[string]packages.PackageManifest, len(pkgs))

	for pkgName, pkg := range pkgs {
		if f.Matches(pkgName, pkg) {
			filtered[pkgName] = pkg
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
