package filter

import (
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/tui"
	"github.com/spf13/cobra"
)

type Filter struct {
	Inputs         map[string]struct{}
	CodeOwners     map[string]struct{}
	KibanaVersions map[string]struct{}
	Categories     map[string]struct{}
}

func NewFilter() *Filter {
	return &Filter{
		Inputs:         make(map[string]struct{}),
		CodeOwners:     make(map[string]struct{}),
		KibanaVersions: make(map[string]struct{}),
		Categories:     make(map[string]struct{}),
	}
}

// splitAndTrim splits a string by delimiter and trims whitespace from each element
func splitAndTrim(s, delimiter string) map[string]struct{} {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, delimiter)
	result := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

// hasAnyMatch checks if any item in the items slice exists in the filters slice
func hasAnyMatch(filters map[string]struct{}, items []string) bool {
	if len(filters) == 0 {
		return true
	}

	for _, item := range items {
		if _, ok := filters[item]; ok {
			return true
		}
	}

	return false
}

func SetFilterFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(cobraext.FilterInputFlagName, "", "", cobraext.FilterInputFlagDescription)
	cmd.Flags().StringP(cobraext.FilterCodeOwnerFlagName, "", "", cobraext.FilterCodeOwnerFlagDescription)
	cmd.Flags().StringP(cobraext.FilterKibanaVersionFlagName, "", "", cobraext.FilterKibanaVersionFlagDescription)
	cmd.Flags().StringP(cobraext.FilterCategoriesFlagName, "", "", cobraext.FilterCategoriesFlagDescription)
}

func Parse(cmd *cobra.Command) (*Filter, error) {
	filter := &Filter{}

	input, err := cmd.Flags().GetString(cobraext.FilterInputFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterInputFlagName)
	}
	filter.Inputs = splitAndTrim(input, ",")

	codeOwner, err := cmd.Flags().GetString(cobraext.FilterCodeOwnerFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterCodeOwnerFlagName)
	}
	filter.CodeOwners = splitAndTrim(codeOwner, ",")

	kibanaVersion, err := cmd.Flags().GetString(cobraext.FilterKibanaVersionFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterKibanaVersionFlagName)
	}
	filter.KibanaVersions = splitAndTrim(kibanaVersion, ",")

	categories, err := cmd.Flags().GetString(cobraext.FilterCategoriesFlagName)
	if err != nil {
		return nil, cobraext.FlagParsingError(err, cobraext.FilterCategoriesFlagName)
	}
	filter.Categories = splitAndTrim(categories, ",")

	return filter, nil
}

func (f *Filter) String() string {
	return fmt.Sprintf(
		"input: %s, codeOwner: %s, kibanaVersion: %s, categories: %s",
		f.Inputs, f.CodeOwners,
		f.KibanaVersions, f.Categories,
	)
}

func (f *Filter) Validate() error {
	validator := tui.Validator{Cwd: "."}

	if len(f.Inputs) == 0 &&
		len(f.CodeOwners) == 0 &&
		len(f.KibanaVersions) == 0 &&
		len(f.Categories) == 0 {
		// No filters provided, return an error
		return fmt.Errorf("at least one flag must be provided")
	}

	if len(f.KibanaVersions) > 0 {
		for version := range f.KibanaVersions {
			if err := validator.Constraint(version); err != nil {
				return fmt.Errorf("invalid kibana version: %w", err)
			}
		}
	}

	if len(f.CodeOwners) > 0 {
		for ownerName := range f.CodeOwners {
			if err := validator.GithubOwner(ownerName); err != nil {
				return fmt.Errorf("invalid code owner: %w", err)
			}
		}
	}

	return nil
}

func (f *Filter) Matches(pkg packages.PackageManifest) bool {
	// Check inputs filter
	if len(f.Inputs) > 0 {
		inputs := f.extractInputs(pkg)
		if !hasAnyMatch(f.Inputs, inputs) {
			return false
		}
	}

	// Check code owners filter
	if len(f.CodeOwners) > 0 {
		if !hasAnyMatch(f.CodeOwners, []string{pkg.Owner.Github}) {
			return false
		}
	}

	// Check categories filter
	if len(f.Categories) > 0 {
		if !hasAnyMatch(f.Categories, pkg.Categories) {
			return false
		}
	}

	// Check kibana version filter
	if len(f.KibanaVersions) > 0 {
		// TODO: Implement kibana version filtering
		// For now, return false to indicate no match
		return false
	}

	return true
}

// extractInputs extracts all input types from package policy templates
func (f *Filter) extractInputs(pkg packages.PackageManifest) []string {
	var inputs []string
	for _, policyTemplate := range pkg.PolicyTemplates {
		if policyTemplate.Input != "" {
			inputs = append(inputs, policyTemplate.Input)
		}
		for _, input := range policyTemplate.Inputs {
			inputs = append(inputs, input.Type)
		}
	}
	return inputs
}

func (f *Filter) ApplyTo(pkgs []packages.PackageManifest) ([]packages.PackageManifest, error) {
	// Pre-allocate with estimated capacity to reduce reallocations
	filteredPackages := make([]packages.PackageManifest, 0, len(pkgs))

	for _, pkg := range pkgs {
		if !f.Matches(pkg) {
			continue
		}
		filteredPackages = append(filteredPackages, pkg)
	}

	return filteredPackages, nil
}
