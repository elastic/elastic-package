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
}

func (f *InputFlag) Name() string {
	return f.name
}

func (f *InputFlag) Description() string {
	return f.description
}

func (f *InputFlag) Shorthand() string {
	return f.shorthand
}

func (f *InputFlag) DefaultValue() string {
	return f.defaultValue
}

func (f *InputFlag) AddToCommand(cmd *cobra.Command) *string {
	return cmd.Flags().StringP(f.Name(), f.Shorthand(), f.DefaultValue(), f.Description())
}

func (f *InputFlag) Parse(cmd *cobra.Command) error {
	return nil
}

func (f *InputFlag) Validate() error {
	return nil
}

func (f *InputFlag) Matches(pkg packages.PackageManifest) bool {
	return true
}

func (f *InputFlag) ApplyTo(pkgs []packages.PackageManifest) ([]packages.PackageManifest, error) {
	return pkgs, nil
}

func setupInputFlag() *InputFlag {
	return &InputFlag{
		name:         cobraext.FilterInputFlagName,
		description:  cobraext.FilterInputFlagDescription,
		shorthand:    "",
		defaultValue: "",
	}
}
