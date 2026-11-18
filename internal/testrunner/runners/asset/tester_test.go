// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

var workflowAssetType = packages.AssetTypeKibanaWorkflow.TypeName()

func TestFindActualAsset(t *testing.T) {
	installed := []packages.Asset{
		{ID: "apache-dashboard-1", Type: "dashboard"},
		{ID: "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-my-pkg-alert-triage", Type: workflowAssetType},
	}

	cases := []struct {
		title    string
		expected packages.Asset
		found    bool
	}{
		{
			title:    "saved object exact match",
			expected: packages.Asset{ID: "apache-dashboard-1", Type: "dashboard"},
			found:    true,
		},
		{
			title:    "saved object suffix doesn't match",
			expected: packages.Asset{ID: "dashboard-1", Type: "dashboard"},
			found:    false,
		},
		{
			title:    "workflow suffix match",
			expected: packages.Asset{ID: "my-pkg-alert-triage", Type: workflowAssetType},
			found:    true,
		},
		{
			title:    "workflow of another package",
			expected: packages.Asset{ID: "other-alert-triage", Type: workflowAssetType},
			found:    false,
		},
		{
			title:    "workflow partial file name",
			expected: packages.Asset{ID: "alert-triage", Type: workflowAssetType},
			found:    true,
		},
		{
			title:    "different type with same ID",
			expected: packages.Asset{ID: "apache-dashboard-1", Type: "visualization"},
			found:    false,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			_, found := findActualAsset(installed, c.expected)
			assert.Equal(t, c.found, found)
		})
	}
}

func TestCheckExpectedAsset(t *testing.T) {
	expected := packages.Asset{ID: "workflows-alert-triage", Type: workflowAssetType}

	cases := []struct {
		title     string
		installed []packages.Asset
		reason    string
	}{
		{
			title: "installed",
			installed: []packages.Asset{
				{ID: "system-fleet-package-workflow-a-b-workflows-alert-triage", Type: workflowAssetType},
			},
		},
		{
			title: "deferred",
			installed: []packages.Asset{
				{ID: "system-fleet-package-workflow-a-b-workflows-alert-triage", Type: workflowAssetType, Deferred: true},
			},
			reason: "asset installation was deferred",
		},
		{
			title:  "missing",
			reason: "could not find expected asset",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			rc := testrunner.NewResultComposer(testrunner.TestResult{TestType: TestType})
			results, err := checkExpectedAsset(rc, c.installed, expected)
			require.NoError(t, err)
			require.Len(t, results, 1)

			if c.reason == "" {
				assert.Empty(t, results[0].FailureMsg)
				return
			}
			assert.Contains(t, results[0].FailureMsg, c.reason)
		})
	}
}

func TestWorkflowAssetsSkip(t *testing.T) {
	cases := []struct {
		version string
		skipped bool
	}{
		{version: "9.6.2", skipped: true},
		{version: "8.19.0", skipped: true},
		{version: "9.7.0", skipped: false},
		{version: "9.8.1", skipped: false},
	}

	for _, c := range cases {
		t.Run(c.version, func(t *testing.T) {
			skip, err := workflowAssetsSkip(kibana.VersionInfo{Number: c.version})
			require.NoError(t, err)
			if c.skipped {
				require.NotNil(t, skip)
				assert.Contains(t, skip.Reason, "9.7.0")
				return
			}
			assert.Nil(t, skip)
		})
	}
}

func TestWorkflowAssetsSkip_InvalidVersion(t *testing.T) {
	_, err := workflowAssetsSkip(kibana.VersionInfo{Number: "invalid"})
	assert.Error(t, err)
}
