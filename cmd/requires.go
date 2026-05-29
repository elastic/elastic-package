// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	updateCmd.Flags().Bool(cobraext.RequiresPrereleaseFlagName, false, cobraext.RequiresPrereleaseFlagDescription)

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
	prerelease, err := cmd.Flags().GetBool(cobraext.RequiresPrereleaseFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.RequiresPrereleaseFlagName)
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
	logger.Debugf("using package registry: %s", baseURL)

	result, err := requiresupdates.Resolve(requiresupdates.Options{
		PackageRoot:    packageRoot,
		RegistryClient: eprClient,
		Prerelease:     prerelease,
	})
	if err != nil {
		return err
	}

	applied := false
	hasBumps := slices.ContainsFunc(result.Proposals, func(p requiresupdates.UpdateProposal) bool {
		return p.Proposed != ""
	})
	if !dryRun && hasBumps {
		manifestPath := filepath.Join(packageRoot, packages.PackageManifestFile)
		manifestBytes, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("reading manifest file failed: %w", err)
		}
		manifestBytes, err = requiresupdates.Apply(manifestBytes, result.Proposals)
		if err != nil {
			return err
		}
		logger.Debugf("writing updated manifest: %s", manifestPath)
		if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
			return fmt.Errorf("writing manifest file failed: %w", err)
		}
		applied = true
	}

	for _, p := range result.Proposals {
		if p.Warning != "" {
			logger.Warn(p.Warning)
		}
	}

	if err := printRequiresUpdateResult(result, os.Stdout, format); err != nil {
		return err
	}

	if format == requiresFormatJSON {
		return nil
	}

	if dryRun && hasBumps {
		cmd.Println("Dry run: manifest.yml was not modified")
	} else if applied {
		cmd.Println("Updated manifest.yml")
	} else if len(result.Proposals) == 0 && result.SkipReason == "" {
		cmd.Println("No dependencies to update")
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
			bold.Fprint(w, "Package: ")     //nolint:errcheck
			fmt.Fprintln(w, result.Package) //nolint:errcheck
		}
		if result.CodeOwner != "" {
			bold.Fprint(w, "Code owner: ")    //nolint:errcheck
			fmt.Fprintln(w, result.CodeOwner) //nolint:errcheck
		}
		if result.SkipReason != "" {
			fmt.Fprintln(w, result.SkipReason) //nolint:errcheck
			return nil
		}
		if len(result.Proposals) == 0 {
			return nil
		}
		bold.Fprintln(w, "Requires updates:") //nolint:errcheck
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
