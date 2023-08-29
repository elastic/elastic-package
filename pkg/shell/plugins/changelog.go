// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/pflag"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
	"github.com/elastic/elastic-package/pkg/shell"
)

var _ shell.Command = changelogCmd{}

type changelogCmd struct{}

func (changelogCmd) Usage() string {
	return "changelog --next {major|minor|patch} --description desc --type {bugfix|enhancement|breaking-change} --link link"
}

func (changelogCmd) Desc() string {
	return "Add an entry to the changelog file in each of the packages in context 'Shell.Packages'."
}

func (changelogCmd) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.String(cobraext.ChangelogAddNextFlagName, "", cobraext.ChangelogAddNextFlagDescription)
	flags.String(cobraext.ChangelogAddDescriptionFlagName, "", cobraext.ChangelogAddDescriptionFlagDescription)
	flags.String(cobraext.ChangelogAddTypeFlagName, "", cobraext.ChangelogAddTypeFlagDescription)
	flags.String(cobraext.ChangelogAddLinkFlagName, "", cobraext.ChangelogAddLinkFlagDescription)
	return flags
}

func (changelogCmd) Exec(ctx context.Context, flags *pflag.FlagSet, args []string, _, stderr io.Writer) (context.Context, error) {
	packages, ok := ctx.Value(ctxKeyPackages).([]string)
	if !ok {
		fmt.Fprintln(stderr, "no packages found in the context")
		return ctx, nil
	}
	for _, pkg := range packages {
		packageRoot := pkg
		// check if we are in packages folder
		if _, err := os.Stat(filepath.Join(".", pkg)); err != nil {
			// check if we are in integrations root folder
			packageRoot = filepath.Join(".", "packages", pkg)
			if _, err := os.Stat(packageRoot); err != nil {
				return ctx, errors.New("you need to be in intgerations root folder or in the packages folder")
			}
		}
		if err := changelogAddCmdForRoot(packageRoot, flags, args); err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func changelogAddCmdForRoot(packageRoot string, flags *pflag.FlagSet, args []string) error {
	nextMode, _ := flags.GetString(cobraext.ChangelogAddNextFlagName)
	v, err := changelogCmdVersion(nextMode, packageRoot)
	if err != nil {
		return err
	}
	version := v.String()

	description, _ := flags.GetString(cobraext.ChangelogAddDescriptionFlagName)
	changeType, _ := flags.GetString(cobraext.ChangelogAddTypeFlagName)
	link, _ := flags.GetString(cobraext.ChangelogAddLinkFlagName)

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

	if err := patchChangelogFile(packageRoot, entry); err != nil {
		return err
	}

	if err := setManifestVersion(packageRoot, version); err != nil {
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
