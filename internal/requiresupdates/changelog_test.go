// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"testing"

	"github.com/stretchr/testify/require"
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
