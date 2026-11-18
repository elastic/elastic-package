// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDashboardsIgnoresNonJSONFiles(t *testing.T) {
	root := t.TempDir()

	workflowDir := filepath.Join(root, "kibana", "workflow")
	require.NoError(t, os.MkdirAll(workflowDir, 0755))
	workflow := []byte("name: sample\nenabled: false\nsteps:\n  - name: hello\n    type: console\n")
	workflowPath := filepath.Join(workflowDir, "pkg-sample.yml")
	require.NoError(t, os.WriteFile(workflowPath, workflow, 0644))

	dashboardDir := filepath.Join(root, "kibana", "dashboard")
	require.NoError(t, os.MkdirAll(dashboardDir, 0755))
	dashboardPath := filepath.Join(dashboardDir, "pkg-dashboard.json")
	require.NoError(t, os.WriteFile(dashboardPath, []byte(`{"attributes":{"optionsJSON":{"a":1}}}`), 0644))

	require.NoError(t, encodeDashboards(root))

	got, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	assert.Equal(t, workflow, got)

	encoded, err := os.ReadFile(dashboardPath)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"optionsJSON": "{\"a\":1}"`)
}
