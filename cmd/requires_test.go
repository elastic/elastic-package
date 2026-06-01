// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
	"github.com/elastic/elastic-package/internal/requiresupdates"
)

func TestRequiresUpdateFlagValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "changelog-type without changelog flag",
			args:        []string{"--changelog-type", "bugfix"},
			errContains: "--changelog",
		},
		{
			name:        "invalid changelog-type value",
			args:        []string{"--changelog", "--changelog-type", "not-valid"},
			errContains: "unsupported changelog type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{RunE: requiresUpdateCommandAction, SilenceErrors: true, SilenceUsage: true}
			cmd.Flags().Bool(cobraext.RequiresDryRunFlagName, false, "")
			cmd.Flags().String(cobraext.RequiresFormatFlagName, requiresFormatTable, "")
			cmd.Flags().Bool(cobraext.RequiresPrereleaseFlagName, false, "")
			cmd.Flags().Bool(cobraext.RequiresChangelogFlagName, false, "")
			cmd.Flags().String(cobraext.RequiresChangelogTypeFlagName, "", "")
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.errContains)
		})
	}
}

const (
	requiresTestBaseManifest = `name: test_pkg
version: "1.2.0"
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
`
	requiresTestMultiDepManifest = `name: test_pkg
version: "1.2.0"
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
  content:
    - package: dashboards
      version: "^0.1.0"
`
	requiresTestBaseChangelog = `- version: "1.2.0"
  changes:
    - description: Initial release.
      type: enhancement
      link: https://github.com/elastic/integrations/pull/1
`
	requiresTestDivergentChangelog = `- version: "1.3.0"
  changes:
    - description: Something.
      type: enhancement
      link: https://github.com/elastic/integrations/pull/2
`
)

func writeRequiresFixture(t *testing.T, manifestContent, changelogContent string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(manifestContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "changelog.yml"), []byte(changelogContent), 0o644))
	return dir
}

func TestApplyRequiresUpdate(t *testing.T) {
	tests := []struct {
		name               string
		manifestContent    string
		changelogContent   string
		proposals          []requiresupdates.UpdateProposal
		changelogEnabled   bool
		changelogType      string
		wantNewVersion     string
		wantManifestVer    string
		wantInputPin       string
		wantContentPin     string
		wantDescription    string
		wantEntryTypes     []string
		wantChangeCount    int
		wantError          string
		// wantFilesUnchanged is valid only for errors that fire before any file
		// write (e.g. the version-mismatch guard). Do not set for errors that
		// occur after PatchYAML has already written changelog.yml.
		wantFilesUnchanged bool
	}{
		{
			name:             "inferred type minor bump",
			manifestContent:  requiresTestBaseManifest,
			changelogContent: requiresTestBaseChangelog,
			proposals: []requiresupdates.UpdateProposal{
				{Kind: requiresupdates.InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.3.0"},
			},
			changelogEnabled: true,
			wantNewVersion:   "1.3.0",
			wantManifestVer:  "1.3.0",
			wantInputPin:     "0.3.0",
			wantDescription:  "Bump `sql_input` input dependency from 0.2.0 to 0.3.0.",
			wantEntryTypes:   []string{"enhancement"},
			wantChangeCount:  1,
		},
		{
			name:             "changelog-type override bugfix",
			manifestContent:  requiresTestBaseManifest,
			changelogContent: requiresTestBaseChangelog,
			proposals: []requiresupdates.UpdateProposal{
				{Kind: requiresupdates.InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.3.0"},
			},
			changelogEnabled: true,
			changelogType:    "bugfix",
			wantNewVersion:   "1.3.0",
			wantManifestVer:  "1.3.0",
			wantInputPin:     "0.3.0",
			wantEntryTypes:   []string{"bugfix"},
			wantChangeCount:  1,
		},
		{
			name:             "major bump breaking-change",
			manifestContent:  requiresTestBaseManifest,
			changelogContent: requiresTestBaseChangelog,
			proposals: []requiresupdates.UpdateProposal{
				{Kind: requiresupdates.InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "1.0.0"},
			},
			changelogEnabled: true,
			wantNewVersion:   "2.0.0",
			wantManifestVer:  "2.0.0",
			wantInputPin:     "1.0.0",
			wantEntryTypes:   []string{"breaking-change"},
			wantChangeCount:  1,
		},
		{
			name:             "multiple bumps aggregate tier",
			manifestContent:  requiresTestMultiDepManifest,
			changelogContent: requiresTestBaseChangelog,
			proposals: []requiresupdates.UpdateProposal{
				{Kind: requiresupdates.InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.2.1"},
				{Kind: requiresupdates.ContentDependency, Package: "dashboards", Current: "^0.1.0", Proposed: "0.5.0"},
			},
			changelogEnabled: true,
			wantNewVersion:   "1.3.0",
			wantManifestVer:  "1.3.0",
			wantInputPin:     "0.2.1",
			wantContentPin:   "0.5.0",
			// patch (sql_input) + minor via constraint (dashboards) → aggregate minor → 1.3.0
			wantEntryTypes:  []string{"enhancement", "enhancement"},
			wantChangeCount: 2,
		},
		{
			name:             "changelog disabled only manifest pin updated",
			manifestContent:  requiresTestBaseManifest,
			changelogContent: requiresTestBaseChangelog,
			proposals: []requiresupdates.UpdateProposal{
				{Kind: requiresupdates.InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.3.0"},
			},
			changelogEnabled: false,
			wantNewVersion:   "",
			wantManifestVer:  "1.2.0",
			wantInputPin:     "0.3.0",
		},
		{
			name:             "divergent manifest vs changelog top",
			manifestContent:  requiresTestBaseManifest,
			changelogContent: requiresTestDivergentChangelog,
			proposals: []requiresupdates.UpdateProposal{
				{Kind: requiresupdates.InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.3.0"},
			},
			changelogEnabled:   true,
			wantError:          "does not match changelog top version",
			wantFilesUnchanged: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeRequiresFixture(t, tc.manifestContent, tc.changelogContent)

			origManifest, err := os.ReadFile(filepath.Join(dir, "manifest.yml"))
			require.NoError(t, err)
			origChangelog, err := os.ReadFile(filepath.Join(dir, "changelog.yml"))
			require.NoError(t, err)

			newVersion, err := applyRequiresUpdate(dir, tc.proposals, tc.changelogEnabled, tc.changelogType)

			if tc.wantError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantError)
				if tc.wantFilesUnchanged {
					gotManifest, err := os.ReadFile(filepath.Join(dir, "manifest.yml"))
					require.NoError(t, err)
					gotChangelog, err := os.ReadFile(filepath.Join(dir, "changelog.yml"))
					require.NoError(t, err)
					require.Equal(t, origManifest, gotManifest, "manifest.yml must be unchanged on error")
					require.Equal(t, origChangelog, gotChangelog, "changelog.yml must be unchanged on error")
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantNewVersion, newVersion)

			pkg, err := packages.ReadPackageManifestFromPackageRoot(dir)
			require.NoError(t, err)
			require.Equal(t, tc.wantManifestVer, pkg.Version)
			if tc.wantInputPin != "" {
				require.NotNil(t, pkg.Requires)
				require.NotEmpty(t, pkg.Requires.Input)
				require.Equal(t, tc.wantInputPin, pkg.Requires.Input[0].Version)
			}
			if tc.wantContentPin != "" {
				require.NotNil(t, pkg.Requires)
				require.NotEmpty(t, pkg.Requires.Content)
				require.Equal(t, tc.wantContentPin, pkg.Requires.Content[0].Version)
			}

			if !tc.changelogEnabled {
				gotChangelog, err := os.ReadFile(filepath.Join(dir, "changelog.yml"))
				require.NoError(t, err)
				require.Equal(t, origChangelog, gotChangelog, "changelog.yml must be unchanged when --changelog is not set")
				return
			}

			revisions, err := changelog.ReadChangelogFromPackageRoot(dir)
			require.NoError(t, err)
			require.Equal(t, tc.wantNewVersion, revisions[0].Version)
			require.Len(t, revisions[0].Changes, tc.wantChangeCount)
			if tc.wantDescription != "" {
				require.Equal(t, tc.wantDescription, revisions[0].Changes[0].Description)
			}
			for i, wantType := range tc.wantEntryTypes {
				require.Equal(t, wantType, revisions[0].Changes[i].Type)
			}
			for _, e := range revisions[0].Changes {
				require.Equal(t, requiresupdates.ChangelogPlaceholderLink, e.Link)
			}
			require.Equal(t, "1.2.0", revisions[1].Version, "original revision must be preserved")
		})
	}
}
