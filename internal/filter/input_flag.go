package filter

import (
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type InputFlag struct {
	FilterFlagBase

	// flag specific fields
	values map[string]struct{}
}

func (f *InputFlag) Parse(cmd *cobra.Command) error {
	input, err := cmd.Flags().GetString(cobraext.FilterInputFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterInputFlagName)
	}
	if input == "" {
		return nil
	}

	f.values = splitAndTrim(input, ",")
	f.isApplied = true
	return nil
}

func (f *InputFlag) Validate() error {
	return nil
}

func (f *InputFlag) Matches(pkgDirName string, pkgManifest packages.PackageManifest) bool {
	if f.values != nil {
		inputs := extractInputs(pkgManifest)
		if !hasAnyMatch(f.values, inputs) {
			return false
		}
	}
	return true
}

func (f *InputFlag) ApplyTo(pkgs map[string]packages.PackageManifest) (map[string]packages.PackageManifest, error) {
	filtered := make(map[string]packages.PackageManifest, len(pkgs))

	for pkgName, pkgManifest := range pkgs {
		if f.Matches(pkgName, pkgManifest) {
			filtered[pkgName] = pkgManifest
		}
	}
	return filtered, nil
}

func initInputFlag() *InputFlag {
	return &InputFlag{
		FilterFlagBase: FilterFlagBase{
			name:         cobraext.FilterInputFlagName,
			description:  cobraext.FilterInputFlagDescription,
			shorthand:    "",
			defaultValue: "",
		},
	}
}
