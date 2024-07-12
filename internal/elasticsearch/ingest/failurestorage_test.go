// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	estest "github.com/elastic/elastic-package/internal/elasticsearch/test"
)

func TestEnableFailureStore(t *testing.T) {
	client := estest.NewClient(t, "testdata/elasticsearch-9-enable-failure-store")

	templateName := "ep-test-index-template"
	templateBody := []byte(`{"index_patterns": ["metrics-eptest.failurestore-*"],"data_stream": {}}`)
	createTempIndexTemplate(t, client.API, templateName, templateBody)
	assertFailureStore(t, client.API, templateName, false)

	err := EnableFailureStore(context.Background(), client.API, templateName, true)
	assert.NoError(t, err)
	assertFailureStore(t, client.API, templateName, true)

	err = EnableFailureStore(context.Background(), client.API, templateName, false)
	assert.NoError(t, err)
	assertFailureStore(t, client.API, templateName, false)
}

func createTempIndexTemplate(t *testing.T, api *elasticsearch.API, name string, body []byte) {
	createResp, err := api.Indices.PutIndexTemplate(name, bytes.NewReader(body),
		api.Indices.PutIndexTemplate.WithCreate(true),
	)
	require.NoError(t, err)
	require.False(t, createResp.IsError(), createResp.String())
	t.Cleanup(func() {
		deleteResp, err := api.Indices.DeleteIndexTemplate(name)
		require.NoError(t, err)
		require.False(t, deleteResp.IsError())
	})
}

func assertFailureStore(t *testing.T, api *elasticsearch.API, name string, expected bool) {
	resp, err := api.Indices.GetIndexTemplate(
		api.Indices.GetIndexTemplate.WithName(name),
	)
	require.NoError(t, err)
	require.False(t, resp.IsError())
	defer resp.Body.Close()

	var templateResponse struct {
		IndexTemplates []struct {
			IndexTemplate struct {
				DataStream struct {
					FailureStore *bool `json:"failure_store"`
				} `json:"data_stream"`
			} `json:"index_template"`
		} `json:"index_templates"`
	}
	err = json.NewDecoder(resp.Body).Decode(&templateResponse)
	require.NoError(t, err)
	require.Len(t, templateResponse.IndexTemplates, 1)
	found := templateResponse.IndexTemplates[0].IndexTemplate.DataStream.FailureStore

	if assert.NotNil(t, found) {
		assert.Equal(t, expected, *found)
	}
}
