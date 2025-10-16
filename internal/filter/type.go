package filter

import (
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type IFilterFlag interface {
	Name() string
	Description() string
	Shorthand() string
	DefaultValue() string

	Register(cmd *cobra.Command)
}

type IFilter interface {
	IFilterFlag

	Parse(cmd *cobra.Command) error
	Validate() error
	Matches(pkg packages.PackageManifest) bool
	ApplyTo(pkgs []packages.PackageManifest) ([]packages.PackageManifest, error)
}

type FilterFlag struct {
	name         string
	description  string
	shorthand    string
	defaultValue string
}

func (f *FilterFlag) Name() string {
	return f.name
}

func (f *FilterFlag) Description() string {
	return f.description
}

func (f *FilterFlag) Shorthand() string {
	return f.shorthand
}

func (f *FilterFlag) DefaultValue() string {
	return f.defaultValue
}

func (f *FilterFlag) Register(cmd *cobra.Command) {
	cmd.Flags().StringP(f.Name(), f.Shorthand(), f.DefaultValue(), f.Description())
}
