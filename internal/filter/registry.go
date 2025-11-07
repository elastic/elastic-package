// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
)

var registry = []Filter{
	initCategoryFlag(),
	initCodeOwnerFlag(),
	initInputFlag(),
	initPackageDirNameFlag(),
	initPackageNameFlag(),
	initPackageTypeFlag(),
	initSpecVersionFlag(),
}

// SetFilterFlags registers all filter flags with the given command.
func SetFilterFlags(cmd *cobra.Command) {
	cmd.Flags().IntP(cobraext.FilterDepthFlagName, cobraext.FilterDepthFlagShorthand, cobraext.FilterDepthFlagDefault, cobraext.FilterDepthFlagDescription)
	cmd.Flags().StringP(cobraext.FilterExcludeDirFlagName, "", "", cobraext.FilterExcludeDirFlagDescription)

	for _, filterFlag := range registry {
		filterFlag.Register(cmd)
	}
}

// FilterRegistry manages a collection of filters for package filtering.
type FilterRegistry struct {
	filters     []Filter
	depth       int
	excludeDirs string
}

// NewFilterRegistry creates a new FilterRegistry instance.
func NewFilterRegistry(depth int, excludeDirs string) *FilterRegistry {
	return &FilterRegistry{
		filters:     []Filter{},
		depth:       depth,
		excludeDirs: excludeDirs,
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
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, multierror.Error{fmt.Errorf("getting current directory failed: %w", err)}
	}

	pkgs, err := packages.ReadAllPackageManifestsFromRepo(currentDir, r.depth, r.excludeDirs)
	if err != nil {
		return nil, multierror.Error{err}
	}

	filtered = pkgs
	for _, filter := range r.filters {
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
