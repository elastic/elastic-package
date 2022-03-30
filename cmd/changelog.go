// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
)

const changelogLongDescription = `Use this command to work with the changelog of the package.`

const changelogAddLongDescription = `Use this command to add an entry to the changelog file.

The entry is added on top of the last entry in the current version. Or optionally in the next
major, minor or patch version, or as a new given version.`

func setupChangelogCommand() *cobraext.Command {
	addChangelogCmd := &cobra.Command{
		Use:   "add",
		Short: "Add an entry to the changelog file",
		Long:  changelogAddLongDescription,
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
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}

	version, _ := cmd.Flags().GetString(cobraext.ChangelogAddVersionFlagName)
	nextMode, _ := cmd.Flags().GetString(cobraext.ChangelogAddNextFlagName)
	if version != "" && nextMode != "" {
		return errors.Errorf("flags %q and %q cannot be used at the same time",
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

	return patchChangelog(packageRoot, entry)
}

func changelogCmdVersion(nextMode, packageRoot string) (*semver.Version, error) {
	revisions, err := changelog.ReadChangelogFromPackageRoot(packageRoot)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read current changelog")
	}
	if len(revisions) == 0 {
		return semver.MustParse("0.0.0"), nil
	}

	version, err := semver.NewVersion(revisions[0].Version)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid version in changelog %q", revisions[0].Version)
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
		return nil, errors.Errorf("invalid value for %q: %s",
			cobraext.ChangelogAddNextFlagName, nextMode)
	}

	return version, nil
}

// patchChangelog looks for the proper place to add the new revision in the changelog,
// trying to conserve original format and comments.
func patchChangelog(packageRoot string, patch changelog.Revision) error {
	changelogPath := filepath.Join(packageRoot, changelog.PackageChangelogFile)
	d, err := ioutil.ReadFile(changelogPath)
	if err != nil {
		return err
	}

	var nodes []yaml.Node
	err = yaml.Unmarshal(d, &nodes)
	if err != nil {
		return err
	}

	patchVersion, err := semver.NewVersion(patch.Version)
	if err != nil {
		return err
	}

	patched := false
	var result []yaml.Node
	for _, node := range nodes {
		if patched {
			result = append(result, node)
			continue
		}

		var entry changelog.Revision
		err := node.Decode(&entry)
		if err != nil {
			result = append(result, node)
			continue
		}

		foundVersion, err := semver.NewVersion(entry.Version)
		if err != nil {
			return err
		}

		var newNode yaml.Node
		if patchVersion.Equal(foundVersion) {
			// Add the change to current entry.
			fmt.Println("Adding changelog entry in version", foundVersion)
			entry.Changes = append(patch.Changes, entry.Changes...)
			err := newNode.Encode(entry)
			if err != nil {
				return err
			}
			fmt.Printf("%+v\n", newNode)
			result = append(result, newNode)
			patched = true
			continue
		}

		// Add the change before first entry
		fmt.Println("Adding changelog entry before version", foundVersion)
		err = newNode.Encode(patch)
		if err != nil {
			return err
		}
		fmt.Printf("%+v\n", newNode)
		// If there is a comment on top, leave it there.
		if node.HeadComment != "" {
			newNode.HeadComment = node.HeadComment
			node.HeadComment = ""
		}
		result = append(result, newNode, node)
		patched = true
	}

	if !patched {
		return errors.New("patch was not applied, this is probably a bug")
	}

	d, err = yaml.Marshal(result)
	if err != nil {
		return errors.Wrap(err, "failed to encode resulting changelog")
	}

	return ioutil.WriteFile(changelogPath, d, 0644)
}
