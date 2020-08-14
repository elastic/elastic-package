package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/packages"
)

// Flag "fail-fast" is used by commands: check, format
const (
	failFastFlagName        = "fail-fast"
	failFastFlagDescription = "fail immediately if any file requires updates"
)

func setupFormatCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Format the package",
		Long:  "Use format command to format the package files.",
		RunE:  formatCommandAction,
	}
	cmd.Flags().BoolP(failFastFlagName, "f", false, failFastFlagDescription)
	return cmd
}

func formatCommandAction(cmd *cobra.Command, args []string) error {
	packageRoot, ok, err := packages.FindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}
	if !ok {
		return errors.New("package root not found")
	}

	ff, err := cmd.Flags().GetBool(failFastFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, failFastFlagName)
	}

	err = formatter.Format(packageRoot, ff)
	if err != nil {
		return errors.Wrapf(err, "formatting the integration failed (path: %s, failFast: %t)", packageRoot, ff)
	}
	return nil
}
