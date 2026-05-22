// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages/requiresupdates"
)

func TestPrintRequiresUpdateResult_JSON(t *testing.T) {
	result := &requiresupdates.Result{
		Package:   "my_pkg",
		CodeOwner: "elastic/my-team",
		Proposals: []requiresupdates.UpdateProposal{
			{
				Kind:             requiresupdates.InputDependency,
				Package:          "sql_input",
				Current:          "0.2.0",
				Proposed:         "0.4.0",
				KibanaConstraint: "^9.4.0",
			},
			{
				Kind:    requiresupdates.ContentDependency,
				Package: "some_content",
				Current: "1.0.0",
				Warning: "newer version 1.1.0 requires ^9.6.0",
			},
		},
		Applied: true,
	}

	var buf bytes.Buffer
	err := printRequiresUpdateResult(result, &buf, requiresFormatJSON)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))

	require.Equal(t, "my_pkg", decoded["package"])
	require.Equal(t, "elastic/my-team", decoded["codeowner"])
	require.True(t, decoded["applied"].(bool))

	proposals, ok := decoded["proposals"].([]any)
	require.True(t, ok)
	require.Len(t, proposals, 2)

	p0 := proposals[0].(map[string]any)
	require.Equal(t, "input", p0["kind"])
	require.Equal(t, "sql_input", p0["package"])
	require.Equal(t, "0.2.0", p0["current"])
	require.Equal(t, "0.4.0", p0["proposed"])
	require.Equal(t, "^9.4.0", p0["kibana_constraint"])
	require.Empty(t, p0["warning"])

	p1 := proposals[1].(map[string]any)
	require.Equal(t, "content", p1["kind"])
	require.Equal(t, "some_content", p1["package"])
	require.Equal(t, "1.0.0", p1["current"])
	require.Contains(t, p1["warning"].(string), "1.1.0")
}

func TestPrintRequiresUpdateResult_JSON_nilResult(t *testing.T) {
	var buf bytes.Buffer
	err := printRequiresUpdateResult(nil, &buf, requiresFormatJSON)
	require.NoError(t, err)
	require.Empty(t, buf.Bytes())
}
