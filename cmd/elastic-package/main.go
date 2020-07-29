package main

import (
	"log"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/install"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "elastic-package",
		Short: "elastic-package - Command line tool for developing Elastic Integrations",
	}
	rootCmd.AddCommand(
		setupBuildCommand(),
		setupClusterCommand(),
		setupTestCommand())

	err := install.EnsureInstalled()
	if err != nil {
		log.Fatal(errors.Wrap(err, "checking installation failed"))
	}

	err = rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
