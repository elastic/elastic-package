// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
)

const changelogLongDescription = `Use this command to work with the changelog of the package.

You can use this command to modify the changelog following the expected format and good practices.
This can be useful when introducing changelog entries for changes done by automated processes.
`

const changelogAddLongDescription = `Use this command to add an entry to the changelog file.

The entry added will include the given description, type and link. It is added on top of the
last entry in the current version

Alternatively, you can start a new version indicating the specific version, or if it should
be the next major, minor or patch version.`

func setupChangelogCommand() *cobraext.Command {
	addChangelogCmd := &cobra.Command{
		Use:   "add",
		Short: "Add an entry to the changelog file",
		Long:  changelogAddLongDescription,
		Args:  cobra.NoArgs,
		RunE:  changelogAddCmd,
	}
	addChangelogCmd.Flags().String(cobraext.ChangelogAddNextFlagName, "", cobraext.ChangelogAddNextFlagDescription)
	addChangelogCmd.Flags().String(cobraext.ChangelogAddVersionFlagName, "", cobraext.ChangelogAddVersionFlagDescription)
	addChangelogCmd.Flags().String(cobraext.ChangelogAddDescriptionFlagName, "", cobraext.ChangelogAddDescriptionFlagDescription)
	addChangelogCmd.MarkFlagRequired(cobraext.ChangelogAddDescriptionFlagName)
	addChangelogCmd.Flags().String(cobraext.ChangelogAddTypeFlagName, "", cobraext.ChangelogAddTypeFlagDescription)
	addChangelogCmd.MarkFlagRequired(cobraext.ChangelogAddTypeFlagName)
	addChangelogCmd.Flags().String(cobraext.ChangelogAddLinkFlagName, "", cobraext.ChangelogAddLinkFlagDescription)
	addChangelogCmd.MarkFlagRequired(cobraext.ChangelogAddLinkFlagName)

	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "Utilities to work with the changelog of the package",
		Long:  changelogLongDescription,
	}
	cmd.AddCommand(addChangelogCmd)

	return cobraext.NewCommand(cmd, cobraext.ContextPackage)
}

func changelogAddCmd(cmd *cobra.Command, args []string) error {
	cwd, err := cobraext.Getwd(cmd)
	if err != nil {
		return err
	}

	packageRoot, err := packages.MustFindPackageRoot(cwd)
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	version, _ := cmd.Flags().GetString(cobraext.ChangelogAddVersionFlagName)
	nextMode, _ := cmd.Flags().GetString(cobraext.ChangelogAddNextFlagName)
	if version != "" && nextMode != "" {
		return fmt.Errorf("flags %q and %q cannot be used at the same time",
			cobraext.ChangelogAddVersionFlagName,
			cobraext.ChangelogAddNextFlagName)
	}
	if version == "" {
		v, err := changelogCmdVersion(nextMode, packageRoot)
		if err != nil {
			return err
		}
		version = v.String()
	}

	description, _ := cmd.Flags().GetString(cobraext.ChangelogAddDescriptionFlagName)
	changeType, _ := cmd.Flags().GetString(cobraext.ChangelogAddTypeFlagName)
	link, _ := cmd.Flags().GetString(cobraext.ChangelogAddLinkFlagName)

	entry := changelog.Revision{
		Version: version,
		Changes: []changelog.Entry{
			{
				Description: description,
				Type:        changeType,
				Link:        link,
			},
		},
	}

	err = patchChangelogFile(packageRoot, entry)
	if err != nil {
		return err
	}

	err = setManifestVersion(packageRoot, version)
	if err != nil {
		return err
	}

	return nil
}

func changelogCmdVersion(nextMode, packageRoot string) (*semver.Version, error) {
	revisions, err := changelog.ReadChangelogFromPackageRoot(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read current changelog: %w", err)
	}
	if len(revisions) == 0 {
		return semver.MustParse("0.0.0"), nil
	}

	version, err := semver.NewVersion(revisions[0].Version)
	if err != nil {
		return nil, fmt.Errorf("invalid version in changelog %q: %w", revisions[0].Version, err)
	}

	switch nextMode {
	case "":
		break
	case "major":
		v := version.IncMajor()
		version = &v
	case "minor":
		v := version.IncMinor()
		version = &v
	case "patch":
		v := version.IncPatch()
		version = &v
	default:
		return nil, fmt.Errorf("invalid value for %q: %s",
			cobraext.ChangelogAddNextFlagName, nextMode)
	}

	return version, nil
}

// patchChangelogFile looks for the proper place to add the new revision in the changelog,
// trying to conserve original format and comments.
func patchChangelogFile(packageRoot string, patch changelog.Revision) error {
	changelogPath := filepath.Join(packageRoot, changelog.PackageChangelogFile)
	d, err := os.ReadFile(changelogPath)
	if err != nil {
		return err
	}

	d, err = changelog.PatchYAML(d, patch)
	if err != nil {
		return err
	}

	return os.WriteFile(changelogPath, d, 0644)
}

func setManifestVersion(packageRoot string, version string) error {
	manifestPath := filepath.Join(packageRoot, packages.PackageManifestFile)
	d, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}

	d, err = changelog.SetManifestVersion(d, version)
	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, d, 0644)
}
