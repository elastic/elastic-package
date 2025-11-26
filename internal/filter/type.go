// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v2"

	"github.com/elastic/elastic-package/internal/packages"
)

// OutputOptions handles both what information to display and how to format it.
type OutputOptions struct {
	infoType string // "package_name", "dir_name", "absolute_path"
	format   string // "json", "yaml", ""
}

// NewOutputOptions creates a new OutputOptions from string parameters.
func NewOutputOptions(infoType, format string) (*OutputOptions, error) {
	cfg := &OutputOptions{
		infoType: infoType,
		format:   format,
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (o *OutputOptions) validate() error {
	validInfo := []string{"package_name", "dir_name", "absolute_path"}
	if !slices.Contains(validInfo, o.infoType) {
		return fmt.Errorf("invalid output info type: %s (valid: %s)", o.infoType, strings.Join(validInfo, ", "))
	}

	validFormats := []string{"json", "yaml", ""}
	if !slices.Contains(validFormats, o.format) {
		return fmt.Errorf("invalid output format: %s (valid: %s)", o.format, strings.Join(validFormats, ", "))
	}

	return nil
}

// ApplyTo applies the output configuration to packages and returns formatted output.
func (o *OutputOptions) ApplyTo(pkgs []packages.PackageDirNameAndManifest) (string, error) {
	if len(pkgs) == 0 {
		return "", nil
	}

	values, err := o.extractInfo(pkgs)
	if err != nil {
		return "", fmt.Errorf("extracting info failed: %w", err)
	}

	// Format output
	return o.formatOutput(values)
}

func (o *OutputOptions) extractInfo(pkgs []packages.PackageDirNameAndManifest) ([]string, error) {

	// Extract information
	values := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		var val string
		switch o.infoType {
		case "package_name":
			val = pkg.Manifest.Name
		case "dir_name":
			val = pkg.DirName
		case "absolute_path":
			val = pkg.Path
		}
		values = append(values, val)
	}

	// Sort for consistent output
	slices.Sort(values)

	return values, nil
}

func (o *OutputOptions) formatOutput(values []string) (string, error) {
	switch o.format {
	case "":
		return strings.Join(values, "\n"), nil
	case "json":
		data, err := json.Marshal(values)
		if err != nil {
			return "", fmt.Errorf("failed to marshal to JSON: %w", err)
		}
		return string(data), nil
	case "yaml":
		data, err := yaml.Marshal(values)
		if err != nil {
			return "", fmt.Errorf("failed to marshal to YAML: %w", err)
		}
		return string(data), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", o.format)
	}
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
	isApplied    bool
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
