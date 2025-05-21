// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v3/cassette"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/stack"
)

// NewClient returns a client for a testing http server that uses prerecorded
// responses. If responses are not found, it forwards the query to the server started by
// elastic-package stack, and records the response.
// Responses are recorded in the directory indicated by serverDataDir.
func NewClient(t *testing.T, recordFileName string, matcher cassette.MatcherFunc) *elasticsearch.Client {
	options, err := clientOptionsForRecord(recordFileName)
	require.NoError(t, err)

	config, err := elasticsearch.NewConfig(options...)
	require.NoError(t, err)

	rec, err := recorder.NewWithOptions(&recorder.Options{
		CassetteName:       recordFileName,
		Mode:               recorder.ModeRecordOnce,
		SkipRequestLatency: true,
		RealTransport:      config.Transport,
	})

	if matcher != nil {
		rec.SetMatcher(matcher)
	}

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

func clientOptionsForRecord(recordFileName string) ([]elasticsearch.ClientOption, error) {
	const defaultAddress = "https://127.0.0.1:9200"
	_, err := os.Stat(cassette.New(recordFileName).File)
	if errors.Is(err, os.ErrNotExist) {
		address := os.Getenv(stack.ElasticsearchHostEnv)
		if address == "" {
			address = defaultAddress
		}
		return []elasticsearch.ClientOption{
			elasticsearch.OptionWithAddress(address),
			elasticsearch.OptionWithPassword(os.Getenv(stack.ElasticsearchPasswordEnv)),
			elasticsearch.OptionWithUsername(os.Getenv(stack.ElasticsearchUsernameEnv)),
			elasticsearch.OptionWithCertificateAuthority(os.Getenv(stack.CACertificateEnv)),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check if record file name exists %s: %w", recordFileName, err)
	}
	options := []elasticsearch.ClientOption{
		elasticsearch.OptionWithAddress(defaultAddress),
	}
	return options, nil
}
