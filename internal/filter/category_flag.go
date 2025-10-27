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

func (f *CategoryFlag) Matches(dirName string, manifest *packages.PackageManifest) bool {
	return hasAnyMatch(f.values, manifest.Categories)
}

func (f *CategoryFlag) ApplyTo(pkgs []packages.PackageDirNameAndManifest) ([]packages.PackageDirNameAndManifest, error) {
	filtered := make([]packages.PackageDirNameAndManifest, 0, len(pkgs))
	for _, pkg := range pkgs {
		if f.Matches(pkg.DirName, pkg.Manifest) {
			filtered = append(filtered, pkg)
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
