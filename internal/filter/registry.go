package filter

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

var registry = []IFilter{
	initCategoryFlag(),
	initCodeOwnerFlag(),
	initInputFlag(),
	initSpecVersionFlag(),
}

func SetFilterFlags(cmd *cobra.Command) {
	for _, filterFlag := range registry {
		filterFlag.Register(cmd)
	}
}

type FilterRegistry struct {
	filters []IFilter
}

func NewFilterRegistry() *FilterRegistry {
	return &FilterRegistry{
		filters: []IFilter{},
	}
}

func (r *FilterRegistry) Parse(cmd *cobra.Command) error {
	errs := multierror.Error{}
	for _, filter := range registry {
		if err := filter.Parse(cmd); err != nil {
			errs = append(errs, err)
		}

		if filter.IsApplied() {
			r.filters = append(r.filters, filter)
		}
	}

	if errs.Error() != "" {
		return fmt.Errorf("error parsing filter options: %s", errs.Error())
	}

	return nil
}

func (r *FilterRegistry) Validate() error {
	for _, filter := range r.filters {
		if err := filter.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (r *FilterRegistry) Execute() (filtered []packages.PackageManifest, err error) {
	root, err := packages.MustFindIntegrationRoot()
	if err != nil {
		return nil, err
	}

	pkgs, err := packages.ReadAllPackageManifests(root)
	if err != nil {
		return nil, err
	}

	filtered = pkgs
	for _, filter := range r.filters {
		filtered, err = filter.ApplyTo(filtered)
		if err != nil || len(filtered) == 0 {
			break
		}
	}

	logger.Infof("Found %d matching package(s)\n", len(filtered))
	return filtered, nil
}
