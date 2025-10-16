package filter

import (
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

var filterRegistry *FilterRegistry = NewFilterRegistry()

func init() {
	filterRegistry.Register(setupInputFlag())
}

func SetFilterFlags(cmd *cobra.Command) {
	for _, filter := range filterRegistry.filters {
		cmd.Flags().StringP(filter.Name(), filter.Shorthand(), filter.DefaultValue(), filter.Description())
	}
}

type FilterRegistry struct {
	filters []FilterImpl
}

func NewFilterRegistry() *FilterRegistry {
	return &FilterRegistry{
		filters: make([]FilterImpl, 0),
	}
}

func (r *FilterRegistry) Register(filter FilterImpl) {
	r.filters = append(r.filters, filter)
}

func (r *FilterRegistry) ApplyTo(pkgs []packages.PackageManifest) (filtered []packages.PackageManifest, err error) {
	filtered = pkgs

	for _, filter := range r.filters {
		filtered, err = filter.ApplyTo(filtered)
		if err != nil {
			return nil, err
		}
	}

	return filtered, nil
}

func (r *FilterRegistry) Execute(pkgs []packages.PackageManifest) (filtered []packages.PackageManifest, err error) {
	return r.ApplyTo(pkgs)
}
