package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/filter"
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
		Use:   "foreach --exec '<subcommand-string>'",
		Short: "Execute a command for filtered packages",
		Long:  foreachLongDescription,
		Example: `  # Run system tests for packages with specific inputs
  elastic-package foreach --exec 'test system -g' --input tcp,udp`,
		RunE: foreachCommandAction,
		Args: cobra.NoArgs,
	}

	// Add filter flags
	filter.SetFilterFlags(cmd)

	// Add pool size flag
	cmd.Flags().IntP(cobraext.ForeachPoolSizeFlagName, "", 1, cobraext.ForeachPoolSizeFlagDescription)

	// Add exec flag and mark it as required
	cmd.Flags().StringP(cobraext.ForeachExecFlagName, "", "", cobraext.ForeachExecFlagDescription)
	cmd.MarkFlagRequired(cobraext.ForeachExecFlagName)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func foreachCommandAction(cmd *cobra.Command, args []string) error {
	exec, err := cmd.Flags().GetString(cobraext.ForeachExecFlagName)
	if err != nil {
		return fmt.Errorf("getting exec failed: %w", err)
	}

	poolSize, err := cmd.Flags().GetInt(cobraext.ForeachPoolSizeFlagName)
	if err != nil {
		return fmt.Errorf("getting pool size failed: %w", err)
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
	fmt.Printf("Found %d matching package(s)\n", len(filtered))

	// Get elastic-package command
	ep := cmd.Parent()

	// Split the exec command string into arguments and append the command arguments
	execArgs := strings.Split(exec, " ")
	newArgs := append(args, execArgs...)
	ep.SetArgs(newArgs)

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	errs := multierror.Error{}

	packageChan := make(chan string, poolSize)

	for range poolSize {
		wg.Add(1)
		go func(packageChan <-chan string) {
			defer wg.Done()
			for packageName := range packageChan {
				if err := executeCommand(ep, args, root, packageName); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("executing command for package %s failed: %w", packageName, err))
					mu.Unlock()
				}
			}
		}(packageChan)
	}

	for _, pkg := range filtered {
		packageChan <- pkg.Name
	}
	close(packageChan)

	wg.Wait()

	if errs.Error() != "" {
		return fmt.Errorf("errors occurred while executing command for packages: \n%s", errs.Error())
	}

	return nil
}

func executeCommand(ep *cobra.Command, args []string, root string, packageName string) error {
	// Set change directory flag to the package directory
	ep.Flags().Set(cobraext.ChangeDirectoryFlagName, filepath.Join(root, "packages", packageName))

	// Execute command
	if err := ep.Execute(); err != nil {
		return fmt.Errorf("executing command for package %s failed: %w", packageName, err)
	}

	return nil
}
