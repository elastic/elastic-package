package filter

import (
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type CategoryFlag struct {
	FilterFlagBase

	values map[string]struct{}
}

func (f *CategoryFlag) Parse(cmd *cobra.Command) error {
	category, err := cmd.Flags().GetString(cobraext.FilterCategoriesFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterCategoriesFlagName)
	}
	if category == "" {
		return nil
	}

	f.values = splitAndTrim(category, ",")
	f.isApplied = true
	return nil
}

func (f *CategoryFlag) Validate() error {
	return nil
}

func (f *CategoryFlag) Matches(pkgDirName string, pkgManifest packages.PackageManifest) bool {
	return hasAnyMatch(f.values, pkgManifest.Categories)
}

func (f *CategoryFlag) ApplyTo(pkgs map[string]packages.PackageManifest) (map[string]packages.PackageManifest, error) {
	filtered := make(map[string]packages.PackageManifest, len(pkgs))
	for pkgDirName, pkgManifest := range pkgs {
		if f.Matches(pkgDirName, pkgManifest) {
			filtered[pkgDirName] = pkgManifest
		}
	}
	return filtered, nil
}

func initCategoryFlag() *CategoryFlag {
	return &CategoryFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterCategoriesFlagName,
			description:  cobraext.FilterCategoriesFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
