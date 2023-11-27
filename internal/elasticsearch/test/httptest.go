// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/stack"
)

// NewClient returns a client for a testing http server that uses prerecorded
// responses. If responses are not found, it forwards the query to the server started by
// elastic-package stack, and records the response.
// Responses are recorded in the directory indicated by serverDataDir.
func NewClient(t *testing.T, serverDataDir string) *elasticsearch.Client {
	address := os.Getenv(stack.ElasticsearchHostEnv)
	if address == "" {
		address = "https://127.0.0.1:9200"
	}
	config, err := elasticsearch.NewConfig(
		elasticsearch.OptionWithAddress(address),
		elasticsearch.OptionWithPassword(os.Getenv(stack.ElasticsearchPasswordEnv)),
		elasticsearch.OptionWithUsername(os.Getenv(stack.ElasticsearchUsernameEnv)),
		elasticsearch.OptionWithCertificateAuthority(os.Getenv(stack.CACertificateEnv)),
	)

	rec, err := recorder.NewWithOptions(&recorder.Options{
		CassetteName:       serverDataDir,
		Mode:               recorder.ModeReplayWithNewEpisodes,
		SkipRequestLatency: false,
		RealTransport:      config.Transport,
	})
	require.NoError(t, err)
	config.Transport = rec

	client, err := elasticsearch.NewClientWithConfig(config)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := rec.Stop()
		require.NoError(t, err)
	})

	return client
}
