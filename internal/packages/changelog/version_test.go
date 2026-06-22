// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package changelog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNextVersionFromRevisions(t *testing.T) {
	rev := func(v string) []Revision { return []Revision{{Version: v}} }

	tests := []struct {
		name      string
		revisions []Revision
		mode      string
		want      string
		wantErr   bool
	}{
		{name: "major", revisions: rev("1.2.3"), mode: "major", want: "2.0.0"},
		{name: "minor", revisions: rev("1.2.3"), mode: "minor", want: "1.3.0"},
		{name: "patch", revisions: rev("1.2.3"), mode: "patch", want: "1.2.4"},
		{name: "empty mode no change", revisions: rev("1.2.3"), mode: "", want: "1.2.3"},
		{name: "empty revisions returns 0.0.0", revisions: nil, mode: "", want: "0.0.0"},
		{name: "empty revisions with patch returns 0.0.1", revisions: nil, mode: "patch", want: "0.0.1"},
		{name: "invalid mode returns error", revisions: rev("1.0.0"), mode: "bogus", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NextVersionFromRevisions(tc.revisions, tc.mode)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got.String())
		})
	}
}
