package main

import (
	"log"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use: "elastic-package",
		Short: "elastic-package - Command line tool for developing Elastic Integrations",
	}
	rootCmd.AddCommand(
		setupClusterCommand(),
		setupTestCommand())

	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
