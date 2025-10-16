package filter

import (
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type FilterImpl interface {
	Register(cmd *cobra.Command)

	Parse(cmd *cobra.Command) error
	Validate() error
	Matches(pkg packages.PackageManifest) bool
	ApplyTo(pkgs []packages.PackageManifest) ([]packages.PackageManifest, error)
}
