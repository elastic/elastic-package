// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import "testing"

func TestAssignedPolicyIDMatches(t *testing.T) {
	tests := []struct {
		name          string
		agentPolicyID string
		policyID      string
		want          bool
	}{
		{
			name:          "exact policy ID",
			agentPolicyID: "19957920-e8d8-4f12-a6e0-2e3f7cff1e6e",
			policyID:      "19957920-e8d8-4f12-a6e0-2e3f7cff1e6e",
			want:          true,
		},
		{
			name:          "version-specific policy ID",
			agentPolicyID: "19957920-e8d8-4f12-a6e0-2e3f7cff1e6e#9.5",
			policyID:      "19957920-e8d8-4f12-a6e0-2e3f7cff1e6e",
			want:          true,
		},
		{
			name:          "different policy ID",
			agentPolicyID: "different-policy-id",
			policyID:      "19957920-e8d8-4f12-a6e0-2e3f7cff1e6e",
			want:          false,
		},
		{
			name:          "same prefix without version separator",
			agentPolicyID: "19957920-e8d8-4f12-a6e0-2e3f7cff1e6e-extra",
			policyID:      "19957920-e8d8-4f12-a6e0-2e3f7cff1e6e",
			want:          false,
		},
		{
			name:          "version separator on different policy ID",
			agentPolicyID: "19957920-e8d8-4f12-a6e0-2e3f7cff1e6ef#9.5",
			policyID:      "19957920-e8d8-4f12-a6e0-2e3f7cff1e6e",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assignedPolicyIDMatches(tt.agentPolicyID, tt.policyID); got != tt.want {
				t.Fatalf("assignedPolicyIDMatches(%q, %q) = %v, want %v", tt.agentPolicyID, tt.policyID, got, tt.want)
			}
		})
	}
}
