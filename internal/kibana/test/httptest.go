// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package test

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v3/cassette"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/stack"
)

// NewClient returns a client for a testing http server that uses prerecorded
// responses. If responses are not found, it forwards the query to the server started by
// elastic-package stack, and records the response.
// Responses are recorded in the directory indicated by serverDataDir.
func NewClient(t *testing.T, recordFileName string) *kibana.Client {
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

	options, err := clientOptionsForRecord(recordFileName)
	require.NoError(t, err)

	options = append(options,
		kibana.HTTPClientSetup(setupHTTPClient),
		kibana.RetryMax(0),
	)

	client, err := kibana.NewClient(options...)
	require.NoError(t, err)

	return client
}

func clientOptionsForRecord(recordFileName string) ([]kibana.ClientOption, error) {
	const defaultAddress = "https://127.0.0.1:5601"
	_, err := os.Stat(cassette.New(recordFileName).File)
	if errors.Is(err, os.ErrNotExist) {
		address := os.Getenv(stack.KibanaHostEnv)
		if address == "" {
			address = defaultAddress
		}
		return []kibana.ClientOption{
			kibana.Address(address),
			kibana.Password(os.Getenv(stack.ElasticsearchPasswordEnv)),
			kibana.Username(os.Getenv(stack.ElasticsearchUsernameEnv)),
			kibana.CertificateAuthority(os.Getenv(stack.CACertificateEnv)),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check if record file name exists %s: %w", recordFileName, err)
	}
	options := []kibana.ClientOption{
		kibana.Address(defaultAddress),
	}
	return options, nil
}
