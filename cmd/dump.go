// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/dump"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/stack"
)

const dumpLongDescription = `Use this command as an exploratory tool to dump resources from Elastic Stack (objects installed as part of package and agent policies).`

const dumpInstalledObjectsLongDescription = `Use this command to dump objects installed by Fleet as part of a package.

Use this command as an exploratory tool to dump objects as they are installed by Fleet when installing a package. Dumped objects are stored in files as they are returned by APIs of the stack, without any processing.`

const dumpAgentPoliciesLongDescription = `Use this command to dump agent policies created by Fleet as part of a package installation.

Use this command as an exploratory tool to dump agent policies as they are created by Fleet when installing a package. Dumped agent policies are stored in files as they are returned by APIs of the stack, without any processing.

If no flag is provided, by default this command dumps all agent policies created by Fleet.

If --package flag is provided, this command dumps all agent policies that the given package has been assigned to it.`

func setupDumpCommand() *cobraext.Command {
	dumpInstalledObjectsCmd := &cobra.Command{
		Use:   "installed-objects",
		Short: "Dump objects installed in the stack",
		Long:  dumpInstalledObjectsLongDescription,
		Args:  cobra.NoArgs,
		RunE:  dumpInstalledObjectsCmdAction,
	}
	dumpInstalledObjectsCmd.Flags().Bool(cobraext.TLSSkipVerifyFlagName, false, cobraext.TLSSkipVerifyFlagDescription)
	dumpInstalledObjectsCmd.Flags().StringP(cobraext.PackageFlagName, cobraext.PackageFlagShorthand, "", cobraext.PackageFlagDescription)
	dumpInstalledObjectsCmd.MarkFlagRequired(cobraext.PackageFlagName)

	dumpAgentPoliciesCmd := &cobra.Command{
		Use:   "agent-policies",
		Short: "Dump agent policies defined in the stack",
		Long:  dumpAgentPoliciesLongDescription,
		Args:  cobra.NoArgs,
		RunE:  dumpAgentPoliciesCmdAction,
	}
	dumpAgentPoliciesCmd.Flags().StringP(cobraext.AgentPolicyFlagName, "", "", cobraext.AgentPolicyDescription)
	dumpAgentPoliciesCmd.Flags().StringP(cobraext.PackageFlagName, cobraext.PackageFlagShorthand, "", cobraext.PackageFlagDescription)

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump package assets",
		Long:  dumpLongDescription,
	}
	cmd.PersistentFlags().StringP(cobraext.DumpOutputFlagName, "o", "package-dump", cobraext.DumpOutputFlagDescription)

	cmd.AddCommand(dumpInstalledObjectsCmd)
	cmd.AddCommand(dumpAgentPoliciesCmd)

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func dumpInstalledObjectsCmdAction(cmd *cobra.Command, args []string) error {
	packageName, err := cmd.Flags().GetString(cobraext.PackageFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageFlagName)
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
	client, err := stack.NewElasticsearchClient(clientOptions...)
	if err != nil {
		return fmt.Errorf("failed to initialize Elasticsearch client: %w", err)
	}

	dumper := dump.NewInstalledObjectsDumper(client.API, packageName)
	n, err := dumper.DumpAll(cmd.Context(), outputPath)
	if err != nil {
		return fmt.Errorf("dump failed: %w", err)
	}
	if n == 0 {
		cmd.Printf("No objects were dumped for package %s, is it installed?\n", packageName)
		return nil
	}
	cmd.Printf("Dumped %d installed objects for package %s to %s\n", n, packageName, outputPath)
	return nil
}

func dumpAgentPoliciesCmdAction(cmd *cobra.Command, args []string) error {
	packageName, err := cmd.Flags().GetString(cobraext.PackageFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.PackageFlagName)
	}

	agentPolicy, err := cmd.Flags().GetString(cobraext.AgentPolicyFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.AgentPolicyFlagName)
	}

	outputPath, err := cmd.Flags().GetString(cobraext.DumpOutputFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.DumpOutputFlagName)
	}

	tlsSkipVerify, _ := cmd.Flags().GetBool(cobraext.TLSSkipVerifyFlagName)

	var clientOptions []kibana.ClientOption
	if tlsSkipVerify {
		clientOptions = append(clientOptions, kibana.TLSSkipVerify())
	}
	kibanaClient, err := stack.NewKibanaClient(clientOptions...)
	if err != nil {
		return fmt.Errorf("failed to initialize Kibana client: %w", err)
	}

	switch {
	case agentPolicy != "" && packageName != "":
		return fmt.Errorf("agent-policy and package parameters cannot be set at the same time")
	case agentPolicy != "":
		dumper := dump.NewAgentPoliciesDumper(kibanaClient)
		err = dumper.DumpByName(cmd.Context(), outputPath, agentPolicy)
		if err != nil {
			return fmt.Errorf("dump failed: %w", err)
		}
		cmd.Printf("Dumped agent policy %s to %s\n", agentPolicy, outputPath)
	case packageName != "":
		dumper := dump.NewAgentPoliciesDumper(kibanaClient)
		count, err := dumper.DumpByPackage(cmd.Context(), outputPath, packageName)
		if err != nil {
			return fmt.Errorf("dump failed: %w", err)
		}
		if count != 0 {
			cmd.Printf("Dumped %d agent policies filtering by package name %s to %s\n", count, packageName, outputPath)
		} else {
			cmd.Printf("No agent policies were found filtering by package name %s\n", packageName)
		}
	default:
		dumper := dump.NewAgentPoliciesDumper(kibanaClient)
		count, err := dumper.DumpAll(cmd.Context(), outputPath)
		if err != nil {
			return fmt.Errorf("dump failed: %w", err)
		}
		if count != 0 {
			cmd.Printf("Dumped %d agent policies to %s\n", count, outputPath)
		} else {
			cmd.Printf("No agent policies were found\n")
		}
	}
	return nil
}
