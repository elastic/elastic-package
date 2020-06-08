package main

import (
	"github.com/elastic/elastic-package/internal/install"
	"github.com/pkg/errors"
	"log"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "elastic-package",
		Short: "elastic-package - Command line tool for developing Elastic Integrations",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := install.EnsureInstalled()
			if err != nil {
				return errors.Wrap(err, "checking installation failed")
			}
			return nil
		},
	}
	rootCmd.AddCommand(
		setupClusterCommand(),
		setupTestCommand())

	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
