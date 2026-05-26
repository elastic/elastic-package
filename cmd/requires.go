// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/registry"
	"github.com/elastic/elastic-package/internal/requiresupdates"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	requiresLongDescription = `Manage requires dependencies for integration packages (requires.input and requires.content in manifest.yml).

Use "requires update" to bump requires.input and requires.content versions from the package registry,
respecting the integration package Kibana version constraint.`

	requiresUpdateLongDescription = `Update requires.input and requires.content pins to the latest registry versions compatible with this package's Kibana constraint.

By default manifest.yml is updated. Use --dry-run to report available bumps without writing the manifest.
Version pins must be exact semver versions (constraints such as ^0.3.0 are not accepted).

When a newer dependency exists but requires a higher Kibana version than this package allows, a warning is printed suggesting to bump conditions.kibana.version on the integration package.`
)

var requiresBold = color.New(color.Bold)

func setupRequiresCommand() *cobraext.Command {
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update requires.input and requires.content versions from the registry",
		Long:  requiresUpdateLongDescription,
		Args:  cobra.NoArgs,
		RunE:  requiresUpdateCommandAction,
	}
	updateCmd.Flags().Bool(cobraext.RequiresDryRunFlagName, false, cobraext.RequiresDryRunFlagDescription)
	updateCmd.Flags().String(cobraext.RequiresFormatFlagName, requiresFormatTable, fmt.Sprintf(cobraext.RequiresFormatFlagDescription, strings.Join(requiresFormatChoices, "|")))

	cmd := &cobra.Command{
		Use:   "requires",
		Short: "Manage requires dependencies for integration packages",
		Long:  requiresLongDescription,
	}
	cmd.AddCommand(updateCmd)
	cmd.PersistentFlags().StringP(cobraext.ProfileFlagName, "p", "", fmt.Sprintf(cobraext.ProfileFlagDescription, install.ProfileNameEnvVar))

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

const (
	requiresFormatTable = "table"
	requiresFormatJSON  = "json"
)

var requiresFormatChoices = []string{requiresFormatTable, requiresFormatJSON}

func requiresUpdateCommandAction(cmd *cobra.Command, _ []string) error {
	dryRun, err := cmd.Flags().GetBool(cobraext.RequiresDryRunFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.RequiresDryRunFlagName)
	}
	format, err := cmd.Flags().GetString(cobraext.RequiresFormatFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.RequiresFormatFlagName)
	}
	if !slices.Contains(requiresFormatChoices, format) {
		return fmt.Errorf("unsupported format %q, supported formats: %s", format, strings.Join(requiresFormatChoices, ", "))
	}

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	prof, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return err
	}

	appConfig, err := install.Configuration()
	if err != nil {
		return fmt.Errorf("loading configuration failed: %w", err)
	}

	baseURL := stack.PackageRegistryBaseURL(prof, appConfig)
	eprClient, err := registry.NewClient(baseURL, stack.RegistryClientOptions(baseURL, prof)...)
	if err != nil {
		return fmt.Errorf("creating package registry client failed: %w", err)
	}

	result, err := requiresupdates.Update(requiresupdates.Options{
		PackageRoot:    packageRoot,
		RegistryClient: eprClient,
		DryRun:         dryRun,
	})
	if err != nil {
		return err
	}

	if result.SkipReason != "" {
		if err := printRequiresUpdateResult(result, os.Stdout, format); err != nil {
			return err
		}
		return nil
	}

	for _, p := range result.Proposals {
		if p.Warning != "" {
			logger.Warn(p.Warning)
		}
	}

	if err := printRequiresUpdateResult(result, os.Stdout, format); err != nil {
		return err
	}

	hasBumps := slices.ContainsFunc(result.Proposals, func(p requiresupdates.UpdateProposal) bool {
		return p.Proposed != ""
	})

	if dryRun && hasBumps {
		cmd.Println("Dry run: manifest.yml was not modified")
	} else if result.Applied {
		cmd.Println("Updated manifest.yml")
	} else if len(result.Proposals) == 0 {
		cmd.Println("Requires dependencies are up to date")
	}

	return nil
}

func printRequiresUpdateResult(result *requiresupdates.Result, w io.Writer, format string) error {
	if result == nil {
		return nil
	}
	switch format {
	case requiresFormatJSON:
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case requiresFormatTable:
		if result.Package != "" {
			requiresBold.Fprint(w, "Package: ") //nolint:errcheck
			fmt.Fprintln(w, result.Package)     //nolint:errcheck
		}
		if result.CodeOwner != "" {
			requiresBold.Fprint(w, "Code owner: ") //nolint:errcheck
			fmt.Fprintln(w, result.CodeOwner)      //nolint:errcheck
		}
		if result.SkipReason != "" {
			fmt.Fprintln(w, result.SkipReason) //nolint:errcheck
			return nil
		}
		if len(result.Proposals) == 0 {
			return nil
		}
		requiresBold.Fprintln(w, "Requires updates:") //nolint:errcheck
		table := tablewriter.NewTable(w,
			tablewriter.WithRenderer(renderer.NewColorized(defaultColorizedConfig())),
			tablewriter.WithConfig(defaultTableConfig),
		)
		table.Header([]string{"Kind", "Dependency", "Current", "Proposed", "Kibana", "Warning"})
		for _, p := range result.Proposals {
			proposed := p.Proposed
			if proposed == "" {
				proposed = "-"
			}
			if err := table.Append([]string{
				string(p.Kind),
				p.Package,
				p.Current,
				proposed,
				p.KibanaConstraint,
				p.Warning,
			}); err != nil {
				return fmt.Errorf("populating requires update table: %w", err)
			}
		}
		return table.Render()
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}
