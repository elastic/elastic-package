// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
)

var registry = []Filter{
	initCategoryFlag(),
	initCodeOwnerFlag(),
	initInputFlag(),
	initPackageNameFlag(),
	initPackageTypeFlag(),
	initSpecVersionFlag(),
}

// SetFilterFlags registers all filter flags with the given command.
func SetFilterFlags(cmd *cobra.Command) {
	for _, filterFlag := range registry {
		filterFlag.Register(cmd)
	}
}

// FilterRegistry manages a collection of filters for package filtering.
type FilterRegistry struct {
	filters []Filter
}

// NewFilterRegistry creates a new FilterRegistry instance.
func NewFilterRegistry() *FilterRegistry {
	return &FilterRegistry{
		filters: []Filter{},
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

func (r *FilterRegistry) Execute() (filtered []packages.PackageDirNameAndManifest, errors multierror.Error) {
	root, err := packages.MustFindIntegrationRoot()
	if err != nil {
		return nil, multierror.Error{err}
	}

	pkgs, err := packages.ReadAllPackageManifests(root)
	if err != nil {
		return nil, multierror.Error{err}
	}

	filtered = pkgs
	for _, filter := range r.filters {
		logger.Infof("Applying for %d packages", len(filtered))
		filtered, err = filter.ApplyTo(filtered)
		if err != nil {
			errors = append(errors, err)
		}

		if len(filtered) == 0 {
			break
		}
	}

	logger.Infof("Found %d matching package(s)\n", len(filtered))
	return filtered, errors
}
