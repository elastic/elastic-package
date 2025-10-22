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
	IsApplied() bool
}

type IFilter interface {
	IFilterFlag

	Parse(cmd *cobra.Command) error
	Validate() error
	ApplyTo(pkgs map[string]packages.PackageManifest) (map[string]packages.PackageManifest, error)
	// pkgDirName is the directory name of the package in package root
	Matches(pkgDirName string, pkgManifest packages.PackageManifest) bool
}

type FilterFlag struct {
	name         string
	description  string
	shorthand    string
	defaultValue string

	isApplied bool
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

func (f *FilterFlag) IsApplied() bool {
	return f.isApplied
}
