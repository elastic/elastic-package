// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
)

var (
	// taken from https://github.com/fatih/color/blob/4d2835ff85a014514ee435d49f76dc8b25c9cee3/color.go#L20-L21
	noColor = os.Getenv("TERM") == "dumb" ||
		(!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()))
)

const (
	productionURL = "https://epr.elastic.co"
	stagingURL    = "https://epr-staging.elastic.co"
	snapshotURL   = "https://epr-snapshot.elastic.co"
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
	cmd.Flags().BoolP(cobraext.NoColorFlagName, "c", false, cobraext.NoColorFlagDescription)
	cmd.Flags().BoolP(cobraext.ShowAllFlagName, "a", false, cobraext.ShowAllFlagDescription)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func statusCommandAction(cmd *cobra.Command, args []string) error {
	var err error
	var status *packageStatus

	showAll, err := cmd.Flags().GetBool(cobraext.ShowAllFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ShowAllFlagName)
	}
	nc, err := cmd.Flags().GetBool(cobraext.NoColorFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.NoColorFlagName)
	}
	if nc {
		noColor = true
	}
	if noColor {
		color.NoColor = true
	}

	if len(args) > 0 {
		status, err = newRemotePackageStatus(args[0], showAll)
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
		status, err = newLocalPackageStatus(packageRootPath, showAll)
		if err != nil {
			return err
		}
	}
	if err := status.print(os.Stdout); err != nil {
		return err
	}
	return nil
}

type packageStatus struct {
	Name       string
	Changelog  []packages.ChangeLogVersion
	Local      *packages.PackageManifest
	Production []packages.PackageManifest
	Staging    []packages.PackageManifest
	Snapshot   []packages.PackageManifest
}

func newLocalPackageStatus(packageRootPath string, showAll bool) (*packageStatus, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return nil, errors.Wrap(err, "reading package manifest failed")
	}
	changelog, err := packages.ReadChangelogFromPackageRoot(packageRootPath)
	if err != nil {
		return nil, errors.Wrap(err, "reading package changelog failed")
	}
	status, err := newRemotePackageStatus(manifest.Name, showAll)
	if err != nil {
		return nil, err
	}
	status.Changelog = changelog
	status.Local = manifest
	return status, nil
}

func newRemotePackageStatus(packageName string, showAll bool) (*packageStatus, error) {
	snapshotManifests, err := getDeployedPackage(packageName, snapshotURL, showAll)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving snapshot deployment failed")
	}
	stagingManifests, err := getDeployedPackage(packageName, stagingURL, showAll)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving staging deployment failed")
	}
	productionManifests, err := getDeployedPackage(packageName, productionURL, showAll)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving production deployment failed")
	}
	return &packageStatus{
		Name:       packageName,
		Snapshot:   snapshotManifests,
		Staging:    stagingManifests,
		Production: productionManifests,
	}, nil
}

func (p *packageStatus) pendingChanges() (*packages.ChangeLogVersion, error) {
	if len(p.Changelog) == 0 || p.Local == nil {
		return nil, nil
	}
	lastChangelogEntry := p.Changelog[0]
	pendingVersion, err := semver.NewVersion(lastChangelogEntry.Version)
	if err != nil {
		return nil, err
	}
	currentVersion, err := semver.NewVersion(p.Local.Version)
	if err != nil {
		return nil, err
	}
	if currentVersion.LessThan(pendingVersion) {
		return &lastChangelogEntry, nil
	}
	return nil, nil
}

func (p *packageStatus) print(w io.Writer) error {
	changes, err := p.pendingChanges()
	if err != nil {
		return errors.Wrap(err, "parsing pending changelog entries failed")
	}
	bold := color.New(color.Bold)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan, color.Bold)
	bold.Fprint(w, "Package: ")
	cyan.Fprintln(w, p.Name)

	if changes != nil {
		bold.Fprint(w, "Next Version: ")
		red.Fprintln(w, changes.Version)
		bold.Fprintln(w, "Pending Changes:")
		changelogTable := [][]string{}
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
	environmentTable := [][]string{}
	if p.Local != nil {
		environmentTable = append(environmentTable, formatManifest("Local", *p.Local, nil))
	}
	environmentTable = append(environmentTable, formatManifests("Snapshot", p.Snapshot))
	environmentTable = append(environmentTable, formatManifests("Staging", p.Staging))
	environmentTable = append(environmentTable, formatManifests("Production", p.Production))
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

func twColor(colors tablewriter.Colors) tablewriter.Colors {
	// this no-ops the color setting if we don't want to colorize the output
	if noColor {
		return tablewriter.Colors{}
	}
	return colors
}

func formatChangelogEntry(change packages.ChangeLogEntry) []string {
	return []string{change.Type, change.Description, change.Link}
}

func formatManifests(environment string, manifests []packages.PackageManifest) []string {
	if len(manifests) == 0 {
		return []string{environment, "-", "-", "-", "-"}
	}
	extraVersions := []string{}
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

func getDeployedPackage(packageName, url string, showAll bool) ([]packages.PackageManifest, error) {
	requestURL := url + "/search?internal=true&experimental=true&package=" + packageName
	if showAll {
		requestURL += "&all=true"
	}
	response, err := http.Get(requestURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	deployedPackageManifests := []packages.PackageManifest{}
	if err := json.Unmarshal(body, &deployedPackageManifests); err != nil {
		return nil, err
	}
	return deployedPackageManifests, nil
}
