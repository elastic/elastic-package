package main

import (
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/install"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "elastic-package",
		Short:        "elastic-package - Command line tool for developing Elastic Integrations",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(
		setupBuildCommand(),
		setupCheckCommand(),
		setupFormatCommand(),
		setupClusterCommand(),
		setupTestCommand(),
		setupValidateCommand())

	err := install.EnsureInstalled()
	if err != nil {
		log.Fatal(errors.Wrap(err, "checking installation failed"))
	}

	err = rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
