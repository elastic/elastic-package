// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/filter"
	"github.com/elastic/elastic-package/internal/packages"
)

const filterLongDescription = `This command gives you a list of all packages based on the given query.

The command will search for packages in the working directory for default depth of 2 and 
return the list of packages that match the given criteria. 

Use --change-directory to change the working directory and --depth to change the depth of the search.`

const filterExample = `elastic-package filter --inputs tcp,udp --categories security --depth 3 --output json
elastic-package filter --packages 'cisco_*,fortinet_*' --output yaml
`

func setupFilterCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:     "filter [flags]",
		Short:   "filter integrations based on given flags",
		Long:    filterLongDescription,
		Args:    cobra.NoArgs,
		RunE:    filterCommandAction,
		Example: filterExample,
	}

	// add filter flags to the command (input, code owner, kibana version, categories)
	filter.SetFilterFlags(cmd)

	// add the output package name and absolute path flags to the command
	cmd.Flags().StringP(cobraext.FilterOutputFlagName, cobraext.FilterOutputFlagShorthand, "", cobraext.FilterOutputFlagDescription)
	cmd.Flags().StringP(cobraext.FilterOutputInfoFlagName, "", cobraext.FilterOutputInfoFlagDefault, cobraext.FilterOutputInfoFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func filterCommandAction(cmd *cobra.Command, args []string) error {
	filtered, err := filterPackage(cmd)
	if err != nil {
		return fmt.Errorf("filtering packages failed: %w", err)
	}

	outputFormatStr, err := cmd.Flags().GetString(cobraext.FilterOutputFlagName)
	if err != nil {
		return fmt.Errorf("getting output format flag failed: %w", err)
	}

	outputInfoStr, err := cmd.Flags().GetString(cobraext.FilterOutputInfoFlagName)
	if err != nil {
		return fmt.Errorf("getting output info flag failed: %w", err)
	}

	outputOptions, err := filter.NewOutputOptions(outputInfoStr, outputFormatStr)
	if err != nil {
		return fmt.Errorf("creating output options failed: %w", err)
	}

	if err = printPkgList(filtered, outputOptions, os.Stdout); err != nil {
		return fmt.Errorf("printing JSON failed: %w", err)
	}

	return nil
}

func filterPackage(cmd *cobra.Command) ([]packages.PackageDirNameAndManifest, error) {
	depth, err := cmd.Flags().GetInt(cobraext.FilterDepthFlagName)
	if err != nil {
		return nil, fmt.Errorf("getting depth flag failed: %w", err)
	}

	excludeDirs, err := cmd.Flags().GetString(cobraext.FilterExcludeDirFlagName)
	if err != nil {
		return nil, fmt.Errorf("getting exclude-dir flag failed: %w", err)
	}

	filters := filter.NewFilterRegistry(depth, excludeDirs)

	if err := filters.Parse(cmd); err != nil {
		return nil, fmt.Errorf("parsing filter options failed: %w", err)
	}

	if err := filters.Validate(); err != nil {
		return nil, fmt.Errorf("validating filter options failed: %w", err)
	}

	filtered, errors := filters.Execute()
	if errors != nil {
		return nil, fmt.Errorf("filtering packages failed: %s", errors.Error())
	}

	return filtered, nil
}

func printPkgList(pkgs []packages.PackageDirNameAndManifest, outputOptions *filter.OutputOptions, w io.Writer) error {
	formatted, err := outputOptions.ApplyTo(pkgs)
	if err != nil {
		return fmt.Errorf("applying output format failed: %w", err)
	}

	// write the formatted packages to the writer
	_, err = io.WriteString(w, formatted+"\n")
	return err
}
