// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import "github.com/Masterminds/semver/v3"

// ChangelogPlaceholderLink is written as the link of every auto-generated
// changelog entry. A follow-up workflow (elastic/integrations#19217 PR-opening
// step) replaces it with the real PR URL. Keep in sync with that workflow.
// NOTE: value is proposed — confirm against the integrations workflow before merge.
const ChangelogPlaceholderLink = "https://github.com/elastic/integrations/pull/REPLACE_ME"

// BumpTier is the semver change tier between a current and proposed version.
type BumpTier int

const (
	TierNone  BumpTier = iota
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

// NextMode maps a tier to the changelog "next" mode string consumed by
// changelogCmdVersion: Major->"major", Minor->"minor", Patch->"patch",
// None->"" (no version change).
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
