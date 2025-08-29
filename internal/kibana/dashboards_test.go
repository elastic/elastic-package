// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
	kibanatest "github.com/elastic/elastic-package/internal/kibana/test"
)

func TestExportDashboards(t *testing.T) {
	cases := []struct {
		record string
	}{
		{
			record: "kibana-8-export-dashboard",
		},
		{
			record: "kibana-9-export-dashboard",
		},
	}

	assertValue := func(t *testing.T, dashboard common.MapStr, key string, expected any) {
		t.Helper()
		value, err := dashboard.GetValue(key)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, expected, value)
	}

	for _, c := range cases {
		t.Run(c.record, func(t *testing.T) {
			client := kibanatest.NewClient(t, filepath.Join("testdata", c.record))
			id := preloadDashboard(t, client)

			dashboardIDs := []string{id}
			dashboards, err := client.Export(t.Context(), dashboardIDs)
			require.NoError(t, err)

			assert.Len(t, dashboards, 1)
			assertValue(t, dashboards[0], "type", "dashboard")
			assertValue(t, dashboards[0], "attributes.title", "Empty Dashboard")
			assertValue(t, dashboards[0], "id", id)
			assertValue(t, dashboards[0], "managed", true)
		})
	}
}
