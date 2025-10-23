// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/filter"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/spf13/cobra"
)

const foreachLongDescription = `Execute a command for each package matching the given filter criteria.

This command combines filtering capabilities with command execution, allowing you to run
any elastic-package subcommand across multiple packages in a single operation.

The command uses the same filter flags as the 'filter' command to select packages, 
then executes the specified subcommand for each matched package.`

func setupForeachCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "foreach [flags] -- <SUBCOMMAND>",
		Short: "Execute a command for filtered packages",
		Long:  foreachLongDescription,
		Example: `  # Run system tests for packages with specific inputs
  elastic-package foreach --input tcp,udp --parallel 10 -- test system -g`,
		RunE: foreachCommandAction,
		Args: cobra.MinimumNArgs(1),
	}

	// Add filter flags
	filter.SetFilterFlags(cmd)

	// Add pool size flag
	cmd.Flags().IntP(cobraext.ForeachPoolSizeFlagName, "", 1, cobraext.ForeachPoolSizeFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func foreachCommandAction(cmd *cobra.Command, args []string) error {
	poolSize, err := cmd.Flags().GetInt(cobraext.ForeachPoolSizeFlagName)
	if err != nil {
		return fmt.Errorf("getting pool size failed: %w", err)
	}

	if err := validateSubCommand(args[0]); err != nil {
		return fmt.Errorf("validating sub command failed: %w", err)
	}

	// Find integration root
	root, err := packages.MustFindIntegrationRoot()
	if err != nil {
		return fmt.Errorf("can't find integration root: %w", err)
	}

	// reuse filterPackage from cmd/filter.go
	filtered, err := filterPackage(cmd)
	if err != nil {
		return fmt.Errorf("filtering packages failed: %w", err)
	}

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	errs := multierror.Error{}
	successes := 0

	packageChan := make(chan string, poolSize)

	for range poolSize {
		wg.Add(1)
		go func(packageChan <-chan string) {
			defer wg.Done()
			for packageName := range packageChan {
				path := filepath.Join(root, "packages", packageName)
				err := executeCommand(args, path)

				mu.Lock()
				if err != nil {
					errs = append(errs, fmt.Errorf("executing command for package %s failed: %w", packageName, err))
				} else {
					successes++
				}
				mu.Unlock()
			}
		}(packageChan)
	}

	for pkgName := range filtered {
		packageChan <- pkgName
	}
	close(packageChan)

	wg.Wait()

	logger.Infof("Successfully executed command for %d packages\n", successes)
	logger.Infof("Errors occurred while executing command for %d packages\n", len(errs))

	if errs.Error() != "" {
		return fmt.Errorf("errors occurred while executing command for packages: \n%s", errs.Error())
	}

	return nil
}

func executeCommand(args []string, path string) error {
	// Look up the elastic-package binary in PATH
	execPath, err := exec.LookPath("elastic-package")
	if err != nil {
		return fmt.Errorf("elastic-package binary not found in PATH: %w", err)
	}

	cmd := &exec.Cmd{
		Path:   execPath,
		Args:   append([]string{execPath}, args...),
		Dir:    path,
		Stdout: io.Discard,
		Stderr: os.Stderr,
		Env:    os.Environ(),
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("executing command for package %s failed: %w", path, err)
	}

	return nil
}

func validateSubCommand(subCommand string) error {
	allowedSubCommands := []string{
		"build",
		"check",
		"clean",
		"format",
		"install",
		"lint",
		"test",
		"uninstall",
	}

	if !slices.Contains(allowedSubCommands, subCommand) {
		return fmt.Errorf("invalid subcommand: %s. Allowed subcommands are: %s", subCommand, strings.Join(allowedSubCommands, ", "))
	}

	return nil
}
