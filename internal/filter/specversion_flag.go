package filter

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type SpecVersionFlag struct {
	FilterFlagBase

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

func (f *SpecVersionFlag) Matches(dirName string, manifest *packages.PackageManifest) bool {
	pkgVersion, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return false
	}
	return f.constraints.Check(pkgVersion)
}

func (f *SpecVersionFlag) ApplyTo(pkgs []packages.PackageDirNameAndManifest) ([]packages.PackageDirNameAndManifest, error) {
	filtered := make([]packages.PackageDirNameAndManifest, 0, len(pkgs))
	for _, pkg := range pkgs {
		if f.Matches(pkg.DirName, pkg.Manifest) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, nil
}

func initSpecVersionFlag() *SpecVersionFlag {
	return &SpecVersionFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterSpecVersionFlagName,
			description:  cobraext.FilterSpecVersionFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
