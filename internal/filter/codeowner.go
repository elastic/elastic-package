package filter

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/tui"
	"github.com/spf13/cobra"
)

type CodeOwnerFlag struct {
	FilterFlag
	values map[string]struct{}
}

func (f *CodeOwnerFlag) Parse(cmd *cobra.Command) error {
	codeOwners, err := cmd.Flags().GetString(cobraext.FilterCodeOwnerFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterCodeOwnerFlagName)
	}
	if codeOwners == "" {
		return nil
	}

	f.values = splitAndTrim(codeOwners, ",")
	f.isApplied = true
	return nil
}

func (f *CodeOwnerFlag) Validate() error {
	validator := tui.Validator{Cwd: "."}

	if f.values != nil {
		for value := range f.values {
			if err := validator.GithubOwner(value); err != nil {
				return fmt.Errorf("invalid code owner: %s: %w", value, err)
			}
		}
	}

	return nil
}

func (f *CodeOwnerFlag) Matches(pkg packages.PackageManifest) bool {
	return hasAnyMatch(f.values, []string{pkg.Owner.Github})
}

func (f *CodeOwnerFlag) ApplyTo(pkgs []packages.PackageManifest) (filtered []packages.PackageManifest, err error) {
	for _, pkg := range pkgs {
		if f.Matches(pkg) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, nil
}

func initCodeOwnerFlag() *CodeOwnerFlag {
	return &CodeOwnerFlag{
		FilterFlag: FilterFlag{
			name:         cobraext.FilterCodeOwnerFlagName,
			description:  cobraext.FilterCodeOwnerFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
