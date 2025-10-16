package filter

import (
	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

type InputFlag struct {
	name         string
	description  string
	shorthand    string
	defaultValue string

	// flag specific fields
	values map[string]struct{}
}

func (f *InputFlag) Register(cmd *cobra.Command) {
	cmd.Flags().StringP(f.name, f.shorthand, f.defaultValue, f.description)
}

func (f *InputFlag) Parse(cmd *cobra.Command) error {
	input, err := cmd.Flags().GetString(cobraext.FilterInputFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.FilterInputFlagName)
	}
	f.values = splitAndTrim(input, ",")
	return nil
}

func (f *InputFlag) Validate() error {
	return nil
}

func (f *InputFlag) Matches(pkg packages.PackageManifest) bool {
	if f.values != nil {
		inputs := extractInputs(pkg)
		if !hasAnyMatch(f.values, inputs) {
			return false
		}
	}
	return true
}

func (f *InputFlag) ApplyTo(pkgs []packages.PackageManifest) (filtered []packages.PackageManifest, err error) {
	for _, pkg := range pkgs {
		if f.Matches(pkg) {
			filtered = append(filtered, pkg)
		}
	}
	return filtered, nil
}

func initInputFlag() *InputFlag {
	return &InputFlag{
		name:         cobraext.FilterInputFlagName,
		description:  cobraext.FilterInputFlagDescription,
		shorthand:    "",
		defaultValue: "",
	}
}
