// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
)

// FilterFlag defines the basic interface for filter flags.
type FilterFlag interface {
	Name() string
	Description() string
	Shorthand() string
	DefaultValue() string

	Register(cmd *cobra.Command)
	IsApplied() bool
}

// Filter extends FilterFlag with filtering capabilities.
// It defines the interface for filtering packages based on specific criteria.
type Filter interface {
	FilterFlag

	Parse(cmd *cobra.Command) error
	Validate() error
	ApplyTo(pkgs []packages.PackageDirNameAndManifest) ([]packages.PackageDirNameAndManifest, error)
	// Matches checks if a package matches the filter criteria.
	// dirName is the directory name of the package in package root.
	Matches(dirName string, manifest *packages.PackageManifest) bool
}

// FilterFlagBase provides common functionality for filter flags.
type FilterFlagBase struct {
	name         string
	description  string
	shorthand    string
	defaultValue string

	isApplied bool
}

func (f *FilterFlagBase) Name() string {
	return f.name
}

func (f *FilterFlagBase) Description() string {
	return f.description
}

func (f *FilterFlagBase) Shorthand() string {
	return f.shorthand
}

func (f *FilterFlagBase) DefaultValue() string {
	return f.defaultValue
}

func (f *FilterFlagBase) Register(cmd *cobra.Command) {
	cmd.Flags().StringP(f.Name(), f.Shorthand(), f.DefaultValue(), f.Description())
}

func (f *FilterFlagBase) IsApplied() bool {
	return f.isApplied
}
