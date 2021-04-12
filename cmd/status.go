// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
	"github.com/elastic/elastic-package/internal/packages/status"
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

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func statusCommandAction(cmd *cobra.Command, args []string) error {
	var err error
	var packageStatus *status.PackageStatus

	showAll, err := cmd.Flags().GetBool(cobraext.ShowAllFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ShowAllFlagName)
	}

	if len(args) > 0 {
		packageStatus, err = status.Package(args[0], showAll)
		if err != nil {
			return err
		}
	} else {
		packageRootPath, found, err := packages.FindPackageRoot()
		if !found {
			return errors.New("no package specified and package root not found")
		}
		if err != nil {
			return errors.Wrap(err, "locating package root failed")
		}
		packageStatus, err = status.LocalPackage(packageRootPath, showAll)
		if err != nil {
			return err
		}
	}
	if err := print(packageStatus, os.Stdout); err != nil {
		return err
	}
	return nil
}

// data printing/formatters

func print(p *status.PackageStatus, w io.Writer) error {
	changes, err := p.PendingChanges()
	if err != nil {
		return errors.Wrap(err, "parsing pending changelog entries failed")
	}
	bold := color.New(color.Bold)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan, color.Bold)
	bold.Fprint(w, "Package: ")
	cyan.Fprintln(w, p.Name)

	var environmentTable [][]string
	if p.Local != nil {
		bold.Fprint(w, "Owner: ")
		owner := "-"
		if p.Local.Owner.Github != "" {
			owner = p.Local.Owner.Github
		}
		cyan.Fprintln(w, owner)

		environmentTable = append(environmentTable, formatManifest("Local", *p.Local, nil))
	}
	environmentTable = append(environmentTable, formatManifests("Snapshot", p.Snapshot))
	environmentTable = append(environmentTable, formatManifests("Staging", p.Staging))
	environmentTable = append(environmentTable, formatManifests("Production", p.Production))

	if changes != nil {
		bold.Fprint(w, "Next Version: ")
		red.Fprintln(w, changes.Version)
		bold.Fprintln(w, "Pending Changes:")
		var changelogTable [][]string
		for _, change := range changes.Changes {
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
	table := tablewriter.NewWriter(os.Stdout)
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
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)
	table.AppendBulk(environmentTable)
	table.Render()
	return nil
}

func formatChangelogEntry(change changelog.Entry) []string {
	return []string{change.Type, change.Description, change.Link}
}

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

func formatManifest(environment string, manifest packages.PackageManifest, extraVersions []string) []string {
	version := manifest.Version
	if len(extraVersions) > 0 {
		version = fmt.Sprintf("%s (%s)", version, strings.Join(extraVersions, ", "))
	}
	return []string{environment, version, manifest.Release, manifest.Title, manifest.Description}
}

func twColor(colors tablewriter.Colors) tablewriter.Colors {
	// this no-ops the color setting if we don't want to colorize the output
	if color.NoColor {
		return tablewriter.Colors{}
	}
	return colors
}
