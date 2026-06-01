// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/cobraext"
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
