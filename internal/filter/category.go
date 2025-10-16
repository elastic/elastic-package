package filter

import (
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type CategoryFlag struct {
	name         string
	description  string
	shorthand    string
	defaultValue string

	values map[string]struct{}
}

func (f *CategoryFlag) Register(cmd *cobra.Command) {
	cmd.Flags().StringP(f.name, f.shorthand, f.defaultValue, f.description)
}

func (f *CategoryFlag) Parse(cmd *cobra.Command) error {
	category, err := cmd.Flags().GetString(cobraext.FilterCategoriesFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterCategoriesFlagName)
	}
	f.values = splitAndTrim(category, ",")
	return nil
}

func (f *CategoryFlag) Validate() error {
	return nil
}

func (f *CategoryFlag) Matches(pkg packages.PackageManifest) bool {
	return hasAnyMatch(f.values, pkg.Categories)
}

func (f *CategoryFlag) ApplyTo(pkgs []packages.PackageManifest) (filtered []packages.PackageManifest, err error) {
	for _, pkg := range pkgs {
		if f.Matches(pkg) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, err
}

func initCategoryFlag() *CategoryFlag {
	return &CategoryFlag{
		name:         cobraext.FilterCategoriesFlagName,
		description:  cobraext.FilterCategoriesFlagDescription,
		shorthand:    "",
		defaultValue: "",
	}
}
