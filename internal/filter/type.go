// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
)

// FilterFlag defines the basic interface for filter flags.
type FilterFlag interface {
	String() string
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

func (f *FilterFlagBase) String() string {
	return fmt.Sprintf("name=%s defaultValue=%s applied=%v", f.name, f.defaultValue, f.isApplied)
}

func (f *FilterFlagBase) Register(cmd *cobra.Command) {
	cmd.Flags().StringP(f.name, f.shorthand, f.defaultValue, f.description)
}

func (f *FilterFlagBase) IsApplied() bool {
	return f.isApplied
}
