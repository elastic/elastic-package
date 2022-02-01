// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/dump"
	"github.com/elastic/elastic-package/internal/elasticsearch"
)

const dumpLongDescription = `Use this command as a exploratory tool to dump assets relevant for the package.`

const dumpInstalledObjectsLongDescription = `Use this command to dump objects installed by Fleet as part of a package.

Use this command as a exploratory tool to dump objects as they are installed by Fleet when installing a package. Dumped objects are stored in files as they are returned by APIs of the stack, without any processing.`

func setupDumpCommand() *cobraext.Command {
	dumpInstalledObjectsCmd := &cobra.Command{
		Use:   "installed-objects",
		Short: "Dump objects installed in the stack",
		Long:  dumpInstalledObjectsLongDescription,
		RunE:  dumpInstalledObjectsCmd,
	}
	dumpInstalledObjectsCmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump package assets",
		Long:  dumpLongDescription,
	}
	cmd.PersistentFlags().StringP(cobraext.DumpPackageFlagName, "p", "", cobraext.DumpPackageFlagDescription)
	cmd.MarkFlagRequired(cobraext.DumpPackageFlagName)
	cmd.PersistentFlags().StringP(cobraext.DumpOutputFlagName, "o", "package-dump", cobraext.DumpOutputFlagDescription)

	cmd.AddCommand(dumpInstalledObjectsCmd)

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func dumpInstalledObjectsCmd(cmd *cobra.Command, args []string) error {
	packageName, err := cmd.Flags().GetString(cobraext.DumpPackageFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.DumpPackageFlagName)
	}

	outputPath, err := cmd.Flags().GetString(cobraext.DumpOutputFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.DumpOutputFlagName)
	}

	tlsSkipVerify, _ := cmd.Flags().GetBool(cobraext.TLSSkipVerifyFlagName)

	var clientOptions []elasticsearch.ClientOption
	if tlsSkipVerify {
		clientOptions = append(clientOptions, elasticsearch.OptionWithSkipTLSVerify())
	}
	client, err := elasticsearch.Client(clientOptions...)
	if err != nil {
		return errors.Wrap(err, "failed to initialize Elasticsearch client")
	}

	dumper := dump.NewInstalledObjectsDumper(client.API, packageName)
	n, err := dumper.DumpAll(cmd.Context(), outputPath)
	if err != nil {
		return errors.Wrap(err, "dump failed")
	}
	if n == 0 {
		cmd.Printf("No objects were dumped for package %s, is it installed?", packageName)
		return nil
	}
	cmd.Printf("Dumped %d installed objects for package %s to %s\n", n, packageName, outputPath)
	return nil
}
