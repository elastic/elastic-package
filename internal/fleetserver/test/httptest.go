// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package test

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v3/cassette"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"

	"github.com/elastic/elastic-package/internal/fleetserver"
)

// NewClient returns a client for a testing http server that uses prerecorded
// responses. If responses are not found, it forwards the query to the server started by
// elastic-package stack, and records the response.
// Responses are recorded in the directory indicated by serverDataDir.
func NewClient(t *testing.T, recordFileName string, host string, options ...fleetserver.ClientOption) *fleetserver.Client {
	setupHTTPClient := func(client *http.Client) *http.Client {
		rec, err := recorder.NewWithOptions(&recorder.Options{
			CassetteName:       recordFileName,
			Mode:               recorder.ModeRecordOnce,
			SkipRequestLatency: true,
			RealTransport:      client.Transport,
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			err := rec.Stop()
			require.NoError(t, err)
		})
		return rec.GetDefaultClient()
	}

	_, err := os.Stat(cassette.New(recordFileName).File)
	if err == nil {
		host = "https://localhost:8220"
		options = nil
	}

	options = append(options, fleetserver.HTTPClientSetup(setupHTTPClient))

	client, err := fleetserver.NewClient(host, options...)
	require.NoError(t, err)

	return client
}
