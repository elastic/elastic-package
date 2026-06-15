// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
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
	"github.com/elastic/elastic-package/internal/formatter"
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

Use --format table (default) or json to control output. JSON includes package, codeowner, and proposals for CI automation; table prints a human-readable summary.
Use --prerelease to include pre-release versions when searching the registry; by default only stable versions are considered.

Use --changelog to add a changelog entry per bumped dependency and bump the package version in manifest.yml and changelog.yml.
The package version is bumped by the largest semver tier across all applied bumps (major over minor over patch).
Use --changelog-type to override the entry type for all generated entries (bugfix, enhancement or breaking-change); by default major bumps map to breaking-change and minor or patch bumps map to enhancement.
--changelog-type requires --changelog.

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
	updateCmd.Flags().Bool(cobraext.RequiresChangelogFlagName, false, cobraext.RequiresChangelogFlagDescription)
	updateCmd.Flags().String(cobraext.RequiresChangelogTypeFlagName, "", fmt.Sprintf(cobraext.RequiresChangelogTypeFlagDescription, strings.Join(updateRequiresChangelogTypeChoices, ", ")))

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

// updateRequiresChangelogTypeChoices is the subset of changelog entry types valid for --changelog-type.
// "deprecation" (valid for `changelog add --type`) is intentionally excluded: it is not a meaningful
// type for an automated dependency-bump entry.
var updateRequiresChangelogTypeChoices = []string{"bugfix", "enhancement", "breaking-change"}

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
	changelogEnabled, err := cmd.Flags().GetBool(cobraext.RequiresChangelogFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.RequiresChangelogFlagName)
	}
	changelogType, err := cmd.Flags().GetString(cobraext.RequiresChangelogTypeFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.RequiresChangelogTypeFlagName)
	}
	if changelogType != "" && !changelogEnabled {
		return fmt.Errorf("--%s requires --%s", cobraext.RequiresChangelogTypeFlagName, cobraext.RequiresChangelogFlagName)
	}
	if changelogType != "" && !slices.Contains(updateRequiresChangelogTypeChoices, changelogType) {
		return fmt.Errorf("unsupported changelog type %q, supported types: %s", changelogType, strings.Join(updateRequiresChangelogTypeChoices, ", "))
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
		newVersion, err := applyRequiresUpdate(packageRoot, result.Proposals, changelogEnabled, changelogType)
		if err != nil {
			return err
		}
		result.NewVersion = newVersion
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
		if changelogEnabled {
			manifestPath := filepath.Join(packageRoot, packages.PackageManifestFile)
			manifestBytes, err := os.ReadFile(manifestPath)
			if err != nil {
				return fmt.Errorf("reading manifest file failed: %w", err)
			}
			plan, err := requiresupdates.PlanChangelog(packageRoot, manifestBytes, result.Proposals, changelogType)
			if err != nil {
				return err
			}
			cmd.Printf("Dry run: would bump package version to %s and add changelog entries:\n", plan.NextVersion)
			for _, e := range plan.Revision.Changes {
				cmd.Printf("  - [%s] %s\n", e.Type, e.Description)
			}
		}
	} else if applied {
		cmd.Println("Updated manifest.yml")
		if changelogEnabled {
			cmd.Println("Updated changelog.yml")
		}
	} else if len(result.Proposals) == 0 && result.SkipReason == "" {
		cmd.Println("No dependencies to update")
	}

	return nil
}

// applyRequiresUpdate orchestrates the manifest write: updates requires pins
// via requiresupdates.Apply, then (when changelogEnabled) patches changelog.yml
// via requiresupdates.ApplyChangelog and bumps the package version via
// requiresupdates.ApplyManifestVersion.
// Returns the new package version when --changelog bumped it ("" otherwise).
// Caller guarantees there is at least one bump (Proposed != "") in proposals.
//
// When --changelog is set, changelog.yml is written before manifest.yml; if
// ApplyManifestVersion or the manifest write fails afterward, changelog.yml may
// be ahead of manifest.yml — the same two-step risk as `elastic-package changelog add`.
func applyRequiresUpdate(packageRoot string, proposals []requiresupdates.UpdateProposal, changelogEnabled bool, changelogType string) (newVersion string, err error) {
	manifestPath := filepath.Join(packageRoot, packages.PackageManifestFile)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", fmt.Errorf("reading manifest file failed: %w", err)
	}

	manifestBytes, err = requiresupdates.Apply(manifestBytes, proposals)
	if err != nil {
		return "", err
	}

	if changelogEnabled {
		newVersion, err = requiresupdates.ApplyChangelog(packageRoot, manifestBytes, proposals, changelogType)
		if err != nil {
			return "", err
		}
		manifestBytes, err = requiresupdates.ApplyManifestVersion(manifestBytes, newVersion)
		if err != nil {
			return "", err
		}
	}

	logger.Debugf("writing updated manifest: %s", manifestPath)
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		return "", fmt.Errorf("writing manifest file failed: %w", err)
	}
	return newVersion, nil
}

func printRequiresUpdateResult(result *requiresupdates.Result, w io.Writer, format string) error {
	if result == nil {
		return nil
	}
	switch format {
	case requiresFormatJSON:
		data, err := formatter.NewJSONFormatter().Encode(result)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(data))
		return err
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
