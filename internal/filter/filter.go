package filter

import (
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type FilterFlagImpl interface {
	Name() string
	Description() string
	Shorthand() string
	DefaultValue() string
}

type FilterImpl interface {
	FilterFlagImpl

	Parse(cmd *cobra.Command) error
	Validate() error
	Matches(pkg packages.PackageManifest) bool
	ApplyTo(pkgs []packages.PackageManifest) ([]packages.PackageManifest, error)
}
