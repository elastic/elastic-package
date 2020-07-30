package main

import (
	"log"
	"os"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/cmd"
	"github.com/elastic/elastic-package/internal/install"
)

func main() {
	rootCmd := cmd.RootCmd()

	err := install.EnsureInstalled()
	if err != nil {
		log.Fatal(errors.Wrap(err, "validating installation failed"))
	}

	err = rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
