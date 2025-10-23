package filter

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/tui"
	"github.com/spf13/cobra"
)

type CodeOwnerFlag struct {
	FilterFlagBase
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

func (f *CodeOwnerFlag) Matches(pkgDirName string, pkgManifest packages.PackageManifest) bool {
	return hasAnyMatch(f.values, []string{pkgManifest.Owner.Github})
}

func (f *CodeOwnerFlag) ApplyTo(pkgs map[string]packages.PackageManifest) (map[string]packages.PackageManifest, error) {
	filtered := make(map[string]packages.PackageManifest, len(pkgs))
	for pkgDirName, pkgManifest := range pkgs {
		if f.Matches(pkgDirName, pkgManifest) {
			filtered[pkgDirName] = pkgManifest
		}
	}
	return filtered, nil
}

func initCodeOwnerFlag() *CodeOwnerFlag {
	return &CodeOwnerFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterCodeOwnerFlagName,
			description:  cobraext.FilterCodeOwnerFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
