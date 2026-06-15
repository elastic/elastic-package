// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages/changelog"
)

// ChangelogPlaceholderLink is written as the link of every auto-generated
// changelog entry. A follow-up workflow (elastic/integrations#19217 PR-opening
// step) replaces it with the real PR URL. Keep in sync with that workflow.
// NOTE: value is proposed — confirm against the integrations workflow before merge.
const ChangelogPlaceholderLink = "https://github.com/elastic/integrations/pull/REPLACE_ME"

// BumpTier is the semver change tier between a current and proposed version.
type BumpTier int

const (
	TierNone BumpTier = iota
	TierPatch
	TierMinor
	TierMajor
)

// Tier classifies the bump from p.Current to p.Proposed.
//
// If p.Current is not an exact semver.Version (e.g. a content-dependency
// constraint like "^0.3.0"), Tier returns TierMinor, because semver/v3 exposes
// no constraint floor to diff against and minor is the safe conservative tier.
// A proposal with an empty Proposed (warning-only) yields TierNone.
func (p UpdateProposal) Tier() BumpTier {
	if p.Proposed == "" {
		return TierNone
	}
	cur, err := semver.NewVersion(p.Current)
	if err != nil {
		return TierMinor
	}
	next, err := semver.NewVersion(p.Proposed)
	if err != nil {
		return TierMinor
	}
	// Use != rather than > so an unexpected downgrade still classifies by the
	// changed component rather than silently returning patch.
	switch {
	case next.Major() != cur.Major():
		return TierMajor
	case next.Minor() != cur.Minor():
		return TierMinor
	default:
		return TierPatch
	}
}

// AggregateTier returns the largest tier across proposals with a non-empty
// Proposed. Returns TierNone when there are none.
func AggregateTier(proposals []UpdateProposal) BumpTier {
	max := TierNone
	for _, p := range proposals {
		if p.Proposed == "" {
			continue
		}
		if t := p.Tier(); t > max {
			max = t
		}
	}
	return max
}

// DefaultChangelogType maps a tier to a changelog entry type:
// major -> "breaking-change"; minor/patch/none -> "enhancement".
// (patch deliberately maps to enhancement; breaking-change is reserved for major.)
func DefaultChangelogType(t BumpTier) string {
	if t == TierMajor {
		return "breaking-change"
	}
	return "enhancement"
}

// NextMode maps a tier to a semver increment keyword:
// Major->"major", Minor->"minor", Patch->"patch", None->"".
func (t BumpTier) NextMode() string {
	switch t {
	case TierMajor:
		return "major"
	case TierMinor:
		return "minor"
	case TierPatch:
		return "patch"
	default:
		return ""
	}
}

// nextVersion reads the changelog top version from packageRoot and returns it
// incremented by tier. Returns 0.0.0 when the changelog is empty.
func nextVersion(tier BumpTier, packageRoot string) (*semver.Version, error) {
	return changelog.NextVersion(packageRoot, tier.NextMode())
}

// assertManifestVersionMatchesChangelogTop errors if the manifest version parsed
// from manifestBytes differs from the changelog top revision version.
func assertManifestVersionMatchesChangelogTop(manifestBytes []byte, revisions []changelog.Revision) error {
	var m struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(manifestBytes, &m); err != nil {
		return fmt.Errorf("parsing manifest version failed: %w", err)
	}
	if len(revisions) == 0 {
		return nil
	}
	manifestVer, err := semver.NewVersion(m.Version)
	if err != nil {
		return fmt.Errorf("invalid manifest version %q: %w", m.Version, err)
	}
	topVer, err := semver.NewVersion(revisions[0].Version)
	if err != nil {
		return fmt.Errorf("invalid changelog top version %q: %w", revisions[0].Version, err)
	}
	if topVer.Prerelease() != "" {
		return fmt.Errorf("changelog top version %s is a pre-release; --changelog is not supported for pre-release packages", topVer)
	}
	if !manifestVer.Equal(topVer) {
		return fmt.Errorf("manifest version %s does not match changelog top version %s; resolve the divergence before running --changelog", m.Version, revisions[0].Version)
	}
	return nil
}

// BuildChangelogRevision constructs a changelog.Revision from proposals.
// Warning-only proposals (empty Proposed) are skipped. overrideType, when
// non-empty, is used for every entry; otherwise DefaultChangelogType is applied
// per proposal tier.
func BuildChangelogRevision(version *semver.Version, proposals []UpdateProposal, overrideType string) changelog.Revision {
	var changes []changelog.Entry
	for _, p := range proposals {
		if p.Proposed == "" {
			continue
		}
		t := overrideType
		if t == "" {
			t = DefaultChangelogType(p.Tier())
		}
		changes = append(changes, changelog.Entry{
			Description: fmt.Sprintf("Bump `%s` %s dependency from %s to %s.", p.Package, p.Kind, p.Current, p.Proposed),
			Type:        t,
			Link:        ChangelogPlaceholderLink,
		})
	}
	return changelog.Revision{Version: version.String(), Changes: changes}
}

// ChangelogPlan holds the computed next version and the revision to be written.
// It is the read-only result of PlanChangelog and is used by both ApplyChangelog
// and the --dry-run preview so they can never disagree.
type ChangelogPlan struct {
	NextVersion *semver.Version
	Revision    changelog.Revision
	// rawChangelogBytes holds the bytes read from changelog.yml by PlanChangelog.
	// ApplyChangelog reuses them for PatchYAML to avoid a second read and the
	// TOCTOU window that a re-read would introduce.
	rawChangelogBytes []byte
}

// PlanChangelog reads changelog.yml from packageRoot, runs the manifest/changelog
// version guards, computes the next package version from the aggregate bump tier,
// and builds the changelog revision. It performs no writes.
// Returns an error if there are no actual bumps (all proposals are warning-only).
func PlanChangelog(packageRoot string, manifestBytes []byte, proposals []UpdateProposal, changelogType string) (*ChangelogPlan, error) {
	tier := AggregateTier(proposals)
	if tier == TierNone {
		return nil, fmt.Errorf("no dependency bumps to write changelog entries for")
	}

	changelogPath := filepath.Join(packageRoot, changelog.PackageChangelogFile)
	changelogBytes, err := os.ReadFile(changelogPath)
	if err != nil {
		return nil, fmt.Errorf("reading changelog file failed: %w", err)
	}
	revisions, err := changelog.ReadChangelogBytes(changelogBytes)
	if err != nil {
		return nil, fmt.Errorf("reading changelog failed: %w", err)
	}

	if err := assertManifestVersionMatchesChangelogTop(manifestBytes, revisions); err != nil {
		return nil, err
	}

	next, err := changelog.NextVersionFromRevisions(revisions, tier.NextMode())
	if err != nil {
		return nil, fmt.Errorf("computing next version failed: %w", err)
	}
	revision := BuildChangelogRevision(next, proposals, changelogType)
	return &ChangelogPlan{NextVersion: next, Revision: revision, rawChangelogBytes: changelogBytes}, nil
}

// ApplyChangelog patches changelog.yml on disk. It is called after Apply has
// already updated the requires pins in manifestBytes. Returns the new package
// version string; use ApplyManifestVersion to bump manifestBytes.
//
// Atomicity: PatchYAML is validated before any file is written; changelog.yml
// is written before the caller writes manifest.yml — the same two-step partial-
// write risk as `elastic-package changelog add`.
func ApplyChangelog(packageRoot string, manifestBytes []byte, proposals []UpdateProposal, changelogType string) (string, error) {
	plan, err := PlanChangelog(packageRoot, manifestBytes, proposals, changelogType)
	if err != nil {
		return "", err
	}

	changelogBytes, err := changelog.PatchYAML(plan.rawChangelogBytes, plan.Revision)
	if err != nil {
		return "", fmt.Errorf("patching changelog failed: %w", err)
	}
	changelogPath := filepath.Join(packageRoot, changelog.PackageChangelogFile)
	if err := os.WriteFile(changelogPath, changelogBytes, 0o644); err != nil {
		return "", fmt.Errorf("writing changelog file failed: %w", err)
	}
	return plan.NextVersion.String(), nil
}
