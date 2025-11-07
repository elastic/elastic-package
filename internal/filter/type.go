// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/packages"
)

type OutputFormat string

const (
	OutputFormatPackageName   OutputFormat = "pkgname"
	OutputFormatDirectoryName OutputFormat = "dirname"
	OutputFormatAbsolutePath  OutputFormat = "absolute"
)

func OutputFormatsList() []OutputFormat {
	return []OutputFormat{OutputFormatPackageName, OutputFormatDirectoryName, OutputFormatAbsolutePath}
}

func (o OutputFormat) String() string {
	return string(o)
}

func NewOutputFormat(s string) (OutputFormat, error) {
	switch s {
	case string(OutputFormatPackageName):
		return OutputFormatPackageName, nil
	case string(OutputFormatDirectoryName):
		return OutputFormatDirectoryName, nil
	case string(OutputFormatAbsolutePath):
		return OutputFormatAbsolutePath, nil
	}
	return "", fmt.Errorf("invalid output format: %s", s)
}

func (o OutputFormat) ApplyTo(pkgs []packages.PackageDirNameAndManifest) ([]string, error) {
	// if no packages are found, return an empty slice
	if len(pkgs) == 0 {
		return nil, nil
	}

	// apply the output format to the packages
	output := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		switch o {
		case OutputFormatPackageName:
			output = append(output, pkg.Manifest.Name)
		case OutputFormatDirectoryName:
			output = append(output, pkg.DirName)
		case OutputFormatAbsolutePath:
			output = append(output, pkg.Path)
		}
	}

	// sort the output
	slices.Sort(output)

	return output, nil
}

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
