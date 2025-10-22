package filter

import (
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type IntegrationTypeFlag struct {
	FilterFlag

	// flag specific fields
	values map[string]struct{}
}

func (f *IntegrationTypeFlag) Parse(cmd *cobra.Command) error {
	integrationTypes, err := cmd.Flags().GetString(cobraext.FilterIntegrationTypeFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterIntegrationTypeFlagName)
	}
	if integrationTypes == "" {
		return nil
	}
	f.values = splitAndTrim(integrationTypes, ",")
	f.isApplied = true
	return nil
}

func (f *IntegrationTypeFlag) Validate() error {
	return nil
}

func (f *IntegrationTypeFlag) Matches(pkgDirName string, pkgManifest packages.PackageManifest) bool {
	if f.values != nil {
		if !hasAnyMatch(f.values, []string{pkgManifest.Type}) {
			return false
		}
	}
	return true
}

func (f *IntegrationTypeFlag) ApplyTo(pkgs map[string]packages.PackageManifest) (map[string]packages.PackageManifest, error) {
	filtered := make(map[string]packages.PackageManifest, len(pkgs))

	for pkgName, pkgManifest := range pkgs {
		if f.Matches(pkgName, pkgManifest) {
			filtered[pkgName] = pkgManifest
		}
	}
	return filtered, nil
}

func initIntegrationTypeFlag() *IntegrationTypeFlag {
	return &IntegrationTypeFlag{
		FilterFlag: FilterFlag{
			name:         cobraext.FilterIntegrationTypeFlagName,
			description:  cobraext.FilterIntegrationTypeFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
