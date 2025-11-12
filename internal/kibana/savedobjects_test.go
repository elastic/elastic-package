// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
	kibanatest "github.com/elastic/elastic-package/internal/kibana/test"
)

func TestSetManagedSavedObject(t *testing.T) {
	// Recorded requests are not going to match the boundaries of
	// multipart fields in requests, but we can ignore it by now as
	// we are mostly interested on the bodies of the responses.
	// To update this test just remove the record file and run the test.
	client := kibanatest.NewClient(t, "testdata/kibana-8-mock-set-managed")

	id := preloadDashboard(t, client)
	require.True(t, getManagedSavedObject(t, client, "dashboard", id))

	err := client.SetManagedSavedObject(t.Context(), "dashboard", id, false)
	require.NoError(t, err)
	assert.False(t, getManagedSavedObject(t, client, "dashboard", id))
}

func preloadDashboard(t *testing.T, client *kibana.Client) string {
	id := "test-managed-saved-objects"
	importRequest := kibana.ImportSavedObjectsRequest{
		Overwrite: false, // We should not need to overwrite objects.
		Objects: []common.MapStr{
			{
				"attributes": map[string]any{
					"title": "Empty Dashboard",
				},
				"managed": true,
				"type":    "dashboard",
				"id":      id,
			},
		},
	}
	_, err := client.ImportSavedObjects(t.Context(), importRequest)
	require.NoError(t, err)

	t.Cleanup(func() {
		statusCode, _, err := client.SendRequest(context.Background(), http.MethodDelete, kibana.SavedObjectsAPI+"/dashboard/"+id, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, statusCode)
	})

	return id
}

func getManagedSavedObject(t *testing.T, client *kibana.Client, savedObjectType string, id string) bool {
	exportRequest := kibana.ExportSavedObjectsRequest{
		ExcludeExportDetails: true,
		Objects: []kibana.ExportSavedObjectsRequestObject{
			{
				ID:   id,
				Type: "dashboard",
			},
		},
	}
	export, err := client.ExportSavedObjects(t.Context(), exportRequest)
	require.NoError(t, err)
	require.Len(t, export, 1)

	managed, found := export[0]["managed"]
	if !found {
		return false
	}

	return managed.(bool)
}
