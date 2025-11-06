// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/stack"
)

var availableServices = map[string]struct{}{
	"elastic-agent":    {},
	"elasticsearch":    {},
	"fleet-server":     {},
	"kibana":           {},
	"package-registry": {},
	"logstash":         {},
}

const stackLongDescription = `Use this command to spin up a Docker-based Elastic Stack consisting of Elasticsearch, Kibana, and the Package Registry. By default the latest released version of the stack is spun up but it is possible to specify a different version, including SNAPSHOT versions by appending --version <version>.

Use --agent-version to specify a different version for the Elastic Agent from the stack.

You can run your own custom images for Elasticsearch, Kibana or Elastic Agent, see [this document](./docs/howto/custom_images.md).

Be aware that a common issue while trying to boot up the stack is that your Docker environments settings are too low in terms of memory threshold.

You can use Podman Desktop instead of Docker, see [this document](./docs/howto/use_podman.md)

For details on how to connect the service with the Elastic stack, see the [service command](https://github.com/elastic/elastic-package/blob/main/README.md#elastic-package-service).`

const stackUpLongDescription = `Use this command to boot up the stack locally.

By default the latest released version of the stack is spun up but it is possible to specify a different version, including SNAPSHOT versions by appending --version <version>.

Use --agent-version to specify a different version for the Elastic Agent from the stack.

You can run your own custom images for Elasticsearch, Kibana or Elastic Agent, see [this document](./docs/howto/custom_images.md).

Be aware that a common issue while trying to boot up the stack is that your Docker environments settings are too low in terms of memory threshold.

You can use Podman Desktop instead of Docker, see [this document](./docs/howto/use_podman.md)

To expose local packages in the Package Registry, build them first and boot up the stack from inside of the Git repository containing the package (e.g. elastic/integrations). They will be copied to the development stack (~/.elastic-package/stack/development) and used to build a custom Docker image of the Package Registry. Starting with Elastic stack version >= 8.7.0, it is not mandatory to be available local packages in the Package Registry to run the tests.

For details on how to connect the service with the Elastic stack, see the [service command](https://github.com/elastic/elastic-package/blob/main/README.md#elastic-package-service).

You can customize your stack using profile settings, see [Elastic Package profiles](https://github.com/elastic/elastic-package/blob/main/README.md#elastic-package-profiles-1) section. These settings can be also overriden with the --parameter flag. Settings configured this way are not persisted.

There are different providers supported, that can be selected with the --provider flag.
- compose: Starts a local stack using Docker Compose. This is the default.
- environment: Prepares an existing stack to be used to test packages. Missing components are started locally using Docker Compose. Environment variables are used to configure the access to the existing Elasticsearch and Kibana instances. You can learn more about this in [this document](./docs/howto/use_existing_stack.md).
- serverless: Uses Elastic Cloud to start a serverless project. Requires an Elastic Cloud API key. You can learn more about this in [this document](./docs/howto/use_serverless_stack.md).`

const stackShellinitLongDescription = `Use this command to export to the current shell the configuration of the stack managed by elastic-package.

The output of this command is intended to be evaluated by the current shell. For example in bash: 'eval $(elastic-package stack shellinit)'.

Relevant environment variables are:

- ELASTIC_PACKAGE_ELASTICSEARCH_API_KEY
- ELASTIC_PACKAGE_ELASTICSEARCH_HOST
- ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME
- ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD
- ELASTIC_PACKAGE_KIBANA_HOST
- ELASTIC_PACKAGE_CA_CERT

You can also provide these environment variables manually. In that case elastic-package commands will use these settings.
`

func setupStackCommand() *cobraext.Command {
	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Boot up the stack",
		Long:  stackUpLongDescription,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Boot up the Elastic stack")

			daemonMode, err := cmd.Flags().GetBool(cobraext.DaemonModeFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.DaemonModeFlagName)
			}

			services, err := cmd.Flags().GetStringSlice(cobraext.StackServicesFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackServicesFlagName)
			}

			common.TrimStringSlice(services)

			err = validateServicesFlag(services)
			if err != nil {
				return fmt.Errorf("validating services failed: %w", err)
			}

			stackVersion, err := cmd.Flags().GetString(cobraext.StackVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackVersionFlagName)
			}

			agentVersion, err := cmd.Flags().GetString(cobraext.AgentVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.AgentVersionFlagName)
			}
			if agentVersion == "" {
				// If agent version is not specified, use the stack version as default
				agentVersion = stackVersion
			}

			profile, err := cobraext.GetProfileFlag(cmd)
			if err != nil {
				return err
			}

			provider, err := cobraext.GetStackProviderFromProfile(cmd, profile, true)
			if err != nil {
				return err
			}

			// Parameters provided through the CLI are not persisted.
			// Stack providers can get them with `profile.Config`, and they
			// need to handle and store them if they need it.
			userParameters, err := cobraext.GetStackUserParameterFlags(cmd)
			if err != nil {
				return err
			}
			profile.RuntimeOverrides(userParameters)

			cmd.Printf("Using profile %s.\n", profile.ProfilePath)
			err = provider.BootUp(cmd.Context(), stack.Options{
				DaemonMode:   daemonMode,
				StackVersion: stackVersion,
				AgentVersion: agentVersion,
				Services:     services,
				Profile:      profile,
				Printer:      cmd,
			})
			if err != nil {
				return fmt.Errorf("booting up the stack failed: %w", err)
			}

			cmd.Println("Done")
			return nil
		},
	}
	upCommand.Flags().BoolP(cobraext.DaemonModeFlagName, "d", false, cobraext.DaemonModeFlagDescription)
	upCommand.Flags().StringSliceP(cobraext.StackServicesFlagName, "s", nil,
		fmt.Sprintf(cobraext.StackServicesFlagDescription, strings.Join(availableServicesAsList(), ",")))
	upCommand.Flags().StringP(cobraext.StackVersionFlagName, "", install.DefaultStackVersion, cobraext.StackVersionFlagDescription)
	upCommand.Flags().String(cobraext.AgentVersionFlagName, "", cobraext.AgentVersionFlagDescription)
	upCommand.Flags().String(cobraext.StackProviderFlagName, "", fmt.Sprintf(cobraext.StackProviderFlagDescription, strings.Join(stack.SupportedProviders, ", ")))
	upCommand.Flags().StringSliceP(cobraext.StackUserParameterFlagName, cobraext.StackUserParameterFlagShorthand, nil, cobraext.StackUserParameterDescription)

	downCommand := &cobra.Command{
		Use:   "down",
		Short: "Take down the stack",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Take down the Elastic stack")

			profile, err := cobraext.GetProfileFlag(cmd)
			if err != nil {
				return err
			}

			provider, err := cobraext.GetStackProviderFromProfile(cmd, profile, false)
			if err != nil {
				return err
			}

			err = provider.TearDown(cmd.Context(), stack.Options{
				Profile: profile,
				Printer: cmd,
			})
			if err != nil {
				return fmt.Errorf("tearing down the stack failed: %w", err)
			}

			cmd.Println("Done")
			return nil
		},
	}

	updateCommand := &cobra.Command{
		Use:   "update",
		Short: "Update the stack to the most recent versions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Update the Elastic stack")

			profile, err := cobraext.GetProfileFlag(cmd)
			if err != nil {
				return err
			}

			provider, err := cobraext.GetStackProviderFromProfile(cmd, profile, false)
			if err != nil {
				return err
			}

			stackVersion, err := cmd.Flags().GetString(cobraext.StackVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackVersionFlagName)
			}

			agentVersion, err := cmd.Flags().GetString(cobraext.AgentVersionFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.AgentVersionFlagName)
			}
			if agentVersion == "" {
				// If agent version is not specified, use the stack version as default
				agentVersion = stackVersion
			}

			err = provider.Update(cmd.Context(), stack.Options{
				StackVersion: stackVersion,
				Profile:      profile,
				Printer:      cmd,
				AgentVersion: agentVersion,
			})
			if err != nil {
				return fmt.Errorf("failed updating the stack images: %w", err)
			}

			cmd.Println("Done")
			return nil
		},
	}
	updateCommand.Flags().StringP(cobraext.StackVersionFlagName, "", install.DefaultStackVersion, cobraext.StackVersionFlagDescription)
	updateCommand.Flags().String(cobraext.AgentVersionFlagName, "", cobraext.AgentVersionFlagDescription)

	shellInitCommand := &cobra.Command{
		Use:   "shellinit",
		Short: "Export environment variables",
		Long:  stackShellinitLongDescription,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			shellName, err := cmd.Flags().GetString(cobraext.ShellInitShellFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.ShellInitShellFlagName)
			}
			if shellName == cobraext.ShellInitShellDetect {
				shellName = stack.AutodetectShell()
				fmt.Fprintf(cmd.OutOrStderr(), "Detected shell: %s\n", shellName)
			}

			profile, err := cobraext.GetProfileFlag(cmd)
			if err != nil {
				return err
			}

			shellCode, err := stack.ShellInit(profile, shellName)
			if err != nil {
				return fmt.Errorf("shellinit failed: %w", err)
			}
			fmt.Println(shellCode)
			return nil
		},
	}
	// NOTE: cobraext.ShellInitShellDetect value is used to trigger automatic detection of parent shell from current process
	shellInitCommand.Flags().StringP(cobraext.ShellInitShellFlagName, "", cobraext.ShellInitShellDetect, cobraext.ShellInitShellDescription)

	dumpCommand := &cobra.Command{
		Use:   "dump",
		Short: "Dump stack data for debug purposes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := cmd.Flags().GetString(cobraext.StackDumpOutputFlagName)
			if err != nil {
				return cobraext.FlagParsingError(err, cobraext.StackDumpOutputFlagName)
			}

			profile, err := cobraext.GetProfileFlag(cmd)
			if err != nil {
				return err
			}

			provider, err := cobraext.GetStackProviderFromProfile(cmd, profile, false)
			if err != nil {
				return err
			}

			target, err := provider.Dump(cmd.Context(), stack.DumpOptions{
				Output:  output,
				Profile: profile,
			})
			if err != nil {
				return fmt.Errorf("dump failed: %w", err)
			}

			cmd.Printf("Path to stack dump: %s\n", target)

			cmd.Println("Done")
			return nil
		},
	}
	dumpCommand.Flags().StringP(cobraext.StackDumpOutputFlagName, "", "elastic-stack-dump", cobraext.StackDumpOutputFlagDescription)

	statusCommand := &cobra.Command{
		Use:   "status",
		Short: "Show status of the stack services",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, err := cobraext.GetProfileFlag(cmd)
			if err != nil {
				return err
			}

			provider, err := cobraext.GetStackProviderFromProfile(cmd, profile, false)
			if err != nil {
				return err
			}

			servicesStatus, err := provider.Status(cmd.Context(), stack.Options{
				Profile: profile,
				Printer: cmd,
			})
			if err != nil {
				return fmt.Errorf("failed getting stack status: %w", err)
			}

			cmd.Println("Status of Elastic stack services:")
			printStatus(cmd, servicesStatus)
			return nil
		},
	}

	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage the Elastic stack",
		Long:  stackLongDescription,
	}
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))
	cmd.AddCommand(
		upCommand,
		downCommand,
		updateCommand,
		shellInitCommand,
		dumpCommand,
		statusCommand)

	return cobraext.NewCommand(cmd, cobraext.ContextGlobal)
}

func availableServicesAsList() []string {
	available := make([]string, len(availableServices))
	i := 0
	for aService := range availableServices {
		available[i] = aService
		i++
	}
	return available
}

func validateServicesFlag(services []string) error {
	selected := map[string]struct{}{}

	for _, aService := range services {
		if _, found := availableServices[aService]; !found {
			return fmt.Errorf("service \"%s\" is not available", aService)
		}

		if _, found := selected[aService]; found {
			return fmt.Errorf("service \"%s\" must be selected at most once", aService)
		}

		selected[aService] = struct{}{}
	}
	return nil
}

func printStatus(cmd *cobra.Command, servicesStatus []stack.ServiceStatus) {
	if len(servicesStatus) == 0 {
		cmd.Printf(" - No service running\n")
		return
	}
	config := defaultColorizedConfig()
	config.Settings.Separators.BetweenRows = tw.Off
	table := tablewriter.NewTable(cmd.OutOrStderr(),
		tablewriter.WithRenderer(renderer.NewColorized(config)),
		tablewriter.WithConfig(defaultTableConfig),
	)
	table.Header("Service", "Version", "Status")
	for _, service := range servicesStatus {
		table.Append(service.Name, service.Version, service.Status)
	}
	table.Render()
}
