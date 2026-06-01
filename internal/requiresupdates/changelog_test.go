// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
)

func TestUpdateProposalTier(t *testing.T) {
	tests := []struct {
		name     string
		proposal UpdateProposal
		want     BumpTier
	}{
		{
			name:     "major bump",
			proposal: UpdateProposal{Current: "1.2.0", Proposed: "2.0.0"},
			want:     TierMajor,
		},
		{
			name:     "minor bump",
			proposal: UpdateProposal{Current: "1.2.0", Proposed: "1.3.0"},
			want:     TierMinor,
		},
		{
			name:     "patch bump",
			proposal: UpdateProposal{Current: "1.2.0", Proposed: "1.2.1"},
			want:     TierPatch,
		},
		{
			name:     "content constraint current falls back to minor",
			proposal: UpdateProposal{Current: "^0.3.0", Proposed: "0.5.0"},
			want:     TierMinor,
		},
		{
			name:     "unparseable current falls back to minor",
			proposal: UpdateProposal{Current: "garbage", Proposed: "1.0.0"},
			want:     TierMinor,
		},
		{
			name:     "empty proposed yields none",
			proposal: UpdateProposal{Current: "1.0.0", Proposed: ""},
			want:     TierNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.proposal.Tier())
		})
	}
}

func TestAggregateTier(t *testing.T) {
	tests := []struct {
		name      string
		proposals []UpdateProposal
		want      BumpTier
	}{
		{
			name: "mixed tiers returns max",
			proposals: []UpdateProposal{
				{Current: "1.2.0", Proposed: "1.2.1"},  // patch
				{Current: "1.0.0", Proposed: "2.0.0"},  // major
				{Current: "1.2.0", Proposed: "1.3.0"},  // minor
			},
			want: TierMajor,
		},
		{
			name: "warning-only proposals ignored",
			proposals: []UpdateProposal{
				{Current: "1.2.0", Proposed: "1.3.0"},          // minor
				{Current: "1.0.0", Proposed: "", Warning: "x"}, // warning-only
			},
			want: TierMinor,
		},
		{
			name:      "all empty proposed returns none",
			proposals: []UpdateProposal{{Current: "1.0.0", Proposed: ""}},
			want:      TierNone,
		},
		{
			name:      "nil slice returns none",
			proposals: nil,
			want:      TierNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, AggregateTier(tc.proposals))
		})
	}
}

func TestDefaultChangelogType(t *testing.T) {
	tests := []struct {
		name string
		tier BumpTier
		want string
	}{
		{name: "major", tier: TierMajor, want: "breaking-change"},
		{name: "minor", tier: TierMinor, want: "enhancement"},
		{name: "patch", tier: TierPatch, want: "enhancement"},
		// TierNone maps to enhancement; there is no changelog entry type for "no change".
		{name: "none", tier: TierNone, want: "enhancement"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, DefaultChangelogType(tc.tier))
		})
	}
}

func TestBumpTierNextMode(t *testing.T) {
	tests := []struct {
		name string
		tier BumpTier
		want string
	}{
		{name: "major", tier: TierMajor, want: "major"},
		{name: "minor", tier: TierMinor, want: "minor"},
		{name: "patch", tier: TierPatch, want: "patch"},
		{name: "none", tier: TierNone, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.tier.NextMode())
		})
	}
}

// --- disk-level tests for ApplyChangelog and NextVersion ---

const baseChangelogFixture = `- version: "1.2.0"
  changes:
    - description: Initial release.
      type: enhancement
      link: https://github.com/elastic/integrations/pull/1
`

func writeChangelogFixture(t *testing.T, manifestContent, changelogContent string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(manifestContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "changelog.yml"), []byte(changelogContent), 0o644))
	return dir
}

func TestApplyChangelog(t *testing.T) {
	baseManifest := `name: test_pkg
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
	tests := []struct {
		name            string
		manifest        string
		changelogYML    string
		proposals       []UpdateProposal
		changelogType   string
		wantNewVersion  string
		wantEntryType   string
		wantError       string
		checkUnchanged  bool
	}{
		{
			name:          "patch bump infers enhancement and bumps patch version",
			manifest:      baseManifest,
			changelogYML:  baseChangelogFixture,
			proposals:     []UpdateProposal{{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.2.1"}},
			wantNewVersion: "1.2.1",
			wantEntryType: "enhancement",
		},
		{
			name:          "minor bump infers enhancement and bumps minor version",
			manifest:      baseManifest,
			changelogYML:  baseChangelogFixture,
			proposals:     []UpdateProposal{{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.3.0"}},
			wantNewVersion: "1.3.0",
			wantEntryType: "enhancement",
		},
		{
			name:          "major bump infers breaking-change and bumps major version",
			manifest:      baseManifest,
			changelogYML:  baseChangelogFixture,
			proposals:     []UpdateProposal{{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "1.0.0"}},
			wantNewVersion: "2.0.0",
			wantEntryType: "breaking-change",
		},
		{
			name:          "changelog-type override applies to all entries",
			manifest:      baseManifest,
			changelogYML:  baseChangelogFixture,
			proposals:     []UpdateProposal{{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.3.0"}},
			changelogType: "bugfix",
			wantNewVersion: "1.3.0",
			wantEntryType: "bugfix",
		},
		{
			name:    "divergent manifest vs changelog top returns error without writing",
			manifest: `name: test_pkg
version: "1.2.0"
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
`,
			changelogYML: `- version: "1.3.0"
  changes:
    - description: Something.
      type: enhancement
      link: https://github.com/elastic/integrations/pull/2
`,
			proposals:      []UpdateProposal{{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.2.1"}},
			wantError:      "does not match changelog top version",
			checkUnchanged: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeChangelogFixture(t, tc.manifest, tc.changelogYML)

			// Apply the pin update first (as the cmd layer does).
			manifestBytes, err := os.ReadFile(filepath.Join(dir, "manifest.yml"))
			require.NoError(t, err)
			manifestBytes, err = Apply(manifestBytes, tc.proposals)
			require.NoError(t, err)

			var origChangelog []byte
			if tc.checkUnchanged {
				origChangelog, err = os.ReadFile(filepath.Join(dir, "changelog.yml"))
				require.NoError(t, err)
			}

			updatedManifest, newVersion, err := ApplyChangelog(dir, manifestBytes, tc.proposals, tc.changelogType)

			if tc.wantError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantError)
				if tc.checkUnchanged {
					gotChangelog, _ := os.ReadFile(filepath.Join(dir, "changelog.yml"))
					require.Equal(t, origChangelog, gotChangelog, "changelog.yml must be unchanged on error")
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantNewVersion, newVersion)

			// Write the returned manifest bytes to disk so we can read them back.
			require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), updatedManifest, 0o644))
			pkg, err := packages.ReadPackageManifestFromPackageRoot(dir)
			require.NoError(t, err)
			require.Equal(t, tc.wantNewVersion, pkg.Version)

			revisions, err := changelog.ReadChangelogFromPackageRoot(dir)
			require.NoError(t, err)
			require.Equal(t, tc.wantNewVersion, revisions[0].Version)
			require.Len(t, revisions[0].Changes, 1)
			require.Equal(t, tc.wantEntryType, revisions[0].Changes[0].Type)
			require.Equal(t, ChangelogPlaceholderLink, revisions[0].Changes[0].Link)
			require.Equal(t, "1.2.0", revisions[1].Version, "original revision must be preserved")
		})
	}
}

func TestApplyChangelogMultipleBumps(t *testing.T) {
	manifest := `name: test_pkg
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
	dir := writeChangelogFixture(t, manifest, baseChangelogFixture)
	proposals := []UpdateProposal{
		{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.2.1"},
		{Kind: ContentDependency, Package: "dashboards", Current: "^0.1.0", Proposed: "0.5.0"},
	}

	manifestBytes, err := os.ReadFile(filepath.Join(dir, "manifest.yml"))
	require.NoError(t, err)
	manifestBytes, err = Apply(manifestBytes, proposals)
	require.NoError(t, err)

	_, newVersion, err := ApplyChangelog(dir, manifestBytes, proposals, "")
	require.NoError(t, err)
	// Content dep current is a constraint so tier=minor; aggregate across patch+minor is minor.
	require.Equal(t, "1.3.0", newVersion)

	revisions, err := changelog.ReadChangelogFromPackageRoot(dir)
	require.NoError(t, err)
	require.Equal(t, "1.3.0", revisions[0].Version)
	require.Len(t, revisions[0].Changes, 2)
	for _, e := range revisions[0].Changes {
		require.Equal(t, ChangelogPlaceholderLink, e.Link)
	}
}
