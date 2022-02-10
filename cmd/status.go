// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
	"github.com/elastic/elastic-package/internal/packages/status"
	"github.com/elastic/elastic-package/internal/registry"
)

const statusLongDescription = `Use this command to display the current deployment status of a package.

If a package name is specified, then information about that package is
returned, otherwise this command checks if the current directory is a
package directory and reports its status.`

func setupStatusCommand() *cobraext.Command {
	cmd := &cobra.Command{
		Use:   "status [package]",
		Short: "Show package status",
		Args:  cobra.MaximumNArgs(1),
		Long:  statusLongDescription,
		RunE:  statusCommandAction,
	}
	cmd.Flags().BoolP(cobraext.ShowAllFlagName, "a", false, cobraext.ShowAllFlagDescription)
	cmd.Flags().String(cobraext.StatusKibanaVersionFlagName, "", cobraext.StatusKibanaVersionFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func statusCommandAction(cmd *cobra.Command, args []string) error {
	var packageName string

	showAll, err := cmd.Flags().GetBool(cobraext.ShowAllFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ShowAllFlagName)
	}
	if len(args) > 0 {
		packageName = args[0]
	}

	kibanaVersion, err := cmd.Flags().GetString(cobraext.StatusKibanaVersionFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.StatusKibanaVersionFlagName)
	}
	options := registry.SearchOptions{
		All:           showAll,
		KibanaVersion: kibanaVersion,
		Prerelease:    true,

		// Deprecated, keeping for compatibility with older versions of the registry.
		Experimental: true,
	}
	packageStatus, err := getPackageStatus(packageName, options)
	if err != nil {
		return err
	}
	return print(packageStatus, os.Stdout)
}

func getPackageStatus(packageName string, options registry.SearchOptions) (*status.PackageStatus, error) {
	if packageName != "" {
		return status.RemotePackage(packageName, options)
	}
	packageRootPath, found, err := packages.FindPackageRoot()
	if !found {
		return nil, errors.New("no package specified and package root not found")
	}
	if err != nil {
		return nil, errors.Wrap(err, "locating package root failed")
	}
	return status.LocalPackage(packageRootPath, options)
}

// print formats and prints package information into a table
func print(p *status.PackageStatus, w io.Writer) error {
	bold := color.New(color.Bold)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan, color.Bold)
	bold.Fprint(w, "Package: ")
	cyan.Fprintln(w, p.Name)

	var environmentTable [][]string
	if p.Local != nil {
		bold.Fprint(w, "Owner: ")
		cyan.Fprintln(w, formatOwner(p))
		environmentTable = append(environmentTable, formatManifest("Local", *p.Local, nil))
	}
	environmentTable = append(environmentTable, formatManifests("Snapshot", p.Snapshot))
	environmentTable = append(environmentTable, formatManifests("Staging", p.Staging))
	environmentTable = append(environmentTable, formatManifests("Production", p.Production))

	if p.PendingChanges != nil {
		bold.Fprint(w, "Next Version: ")
		red.Fprintln(w, p.PendingChanges.Version)
		bold.Fprintln(w, "Pending Changes:")
		var changelogTable [][]string
		for _, change := range p.PendingChanges.Changes {
			changelogTable = append(changelogTable, formatChangelogEntry(change))
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Type", "Description", "Link"})
		table.SetHeaderColor(
			twColor(tablewriter.Colors{tablewriter.Bold}),
			twColor(tablewriter.Colors{tablewriter.Bold}),
			twColor(tablewriter.Colors{tablewriter.Bold}),
		)
		table.SetColumnColor(
			twColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}),
			tablewriter.Colors{},
			tablewriter.Colors{},
		)
		table.SetRowLine(true)
		table.AppendBulk(changelogTable)
		table.Render()
	}

	bold.Fprintln(w, "Package Versions:")
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Environment", "Version", "Release", "Title", "Description"})
	table.SetHeaderColor(
		twColor(tablewriter.Colors{tablewriter.Bold}),
		twColor(tablewriter.Colors{tablewriter.Bold}),
		twColor(tablewriter.Colors{tablewriter.Bold}),
		twColor(tablewriter.Colors{tablewriter.Bold}),
		twColor(tablewriter.Colors{tablewriter.Bold}),
	)
	table.SetColumnColor(
		twColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor}),
		twColor(tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor}),
		tablewriter.Colors{},
		tablewriter.Colors{},
		tablewriter.Colors{},
	)
	table.SetRowLine(true)
	table.AppendBulk(environmentTable)
	table.Render()
	return nil
}

// formatOwner returns the name of the package owner
func formatOwner(p *status.PackageStatus) string {
	if p.Local != nil && p.Local.Owner.Github != "" {
		return p.Local.Owner.Github
	}
	return "-"
}

// formatChangelogEntry returns a row of changelog data
func formatChangelogEntry(change changelog.Entry) []string {
	return []string{change.Type, change.Description, change.Link}
}

// formatManifests returns a row of data ffor a set of versioned packaged manifests
func formatManifests(environment string, manifests []packages.PackageManifest) []string {
	if len(manifests) == 0 {
		return []string{environment, "-", "-", "-", "-"}
	}
	var extraVersions []string
	for i, m := range manifests {
		if i != len(manifests)-1 {
			extraVersions = append(extraVersions, m.Version)
		}
	}
	return formatManifest(environment, manifests[len(manifests)-1], extraVersions)
}

// formatManifest returns a row of data for a given package manifest
func formatManifest(environment string, manifest packages.PackageManifest, extraVersions []string) []string {
	version := manifest.Version
	if len(extraVersions) > 0 {
		version = fmt.Sprintf("%s (%s)", version, strings.Join(extraVersions, ", "))
	}
	return []string{environment, version, releaseFromVersion(manifest.Version), manifest.Title, manifest.Description}
}

// twColor no-ops the color setting if we don't want to colorize the output
func twColor(colors tablewriter.Colors) tablewriter.Colors {
	if color.NoColor {
		return tablewriter.Colors{}
	}
	return colors
}

// releaseFromVersion returns the human-friendly release level based on semantic versioning conventions.
// It does a best-effort mapping, it doesn't do validation.
func releaseFromVersion(version string) string {
	const (
		previewVersionText   = "Technical Preview"
		betaVersionText      = "Beta"
		releaseCandidateText = "Release Candidate"
		gaVersion            = "GA"
		defaultText          = betaVersionText
	)

	conventionPrereleasePrefixes := []struct {
		prefix string
		text   string
	}{
		{"beta", betaVersionText},
		{"rc", releaseCandidateText},
		{"preview", previewVersionText},
	}

	sv, err := semver.NewVersion(version)
	if err != nil {
		// Ignoring errors on version parsing here, use best-effort defaults.
		if strings.HasPrefix(version, "0.") {
			return previewVersionText
		}
		return defaultText
	}

	if sv.Major() == 0 {
		return previewVersionText
	}
	if sv.Prerelease() == "" {
		return gaVersion
	}

	for _, convention := range conventionPrereleasePrefixes {
		if strings.HasPrefix(sv.Prerelease(), convention.prefix) {
			return convention.text
		}
	}

	return defaultText
}
