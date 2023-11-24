// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/stack"
)

func TestSetManagedSavedObject(t *testing.T) {
	// TODO: Use kibana test client when we support recording POST requests.
	client, err := stack.NewKibanaClient(kibana.RetryMax(0))
	var undefinedEnvError *stack.ErrUndefinedEnv
	if errors.As(err, &undefinedEnvError) {
		t.Skip("Kibana host required:", err)
	}
	require.NoError(t, err)

	id := preloadDashboard(t, client)
	require.True(t, getManagedSavedObject(t, client, "dashboard", id))

	err = client.SetManagedSavedObject("dashboard", id, false)
	require.NoError(t, err)
	assert.False(t, getManagedSavedObject(t, client, "dashboard", id))
}

func preloadDashboard(t *testing.T, client *kibana.Client) string {
	id := uuid.New().String()
	importRequest := kibana.ImportSavedObjectsRequest{
		Overwrite: false, // Highly unlikely, but avoid overwriting existing objects.
		Objects: []map[string]any{
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
	_, err := client.ImportSavedObjects(importRequest)
	require.NoError(t, err)

	t.Cleanup(func() {
		statusCode, _, err := client.SendRequest(http.MethodDelete, kibana.SavedObjectsAPI+"/dashboard/"+id, nil)
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
	export, err := client.ExportSavedObjects(exportRequest)
	require.NoError(t, err)
	require.Len(t, export, 1)

	managed, found := export[0]["managed"]
	if !found {
		return false
	}

	return managed.(bool)
}
