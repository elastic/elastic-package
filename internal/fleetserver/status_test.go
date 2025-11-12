// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fleetserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/fleetserver"
	fleetservertest "github.com/elastic/elastic-package/internal/fleetserver/test"
)

func TestStatusAPIKeyAuthenticated(t *testing.T) {
	client := fleetservertest.NewClient(t,
		"./testdata/status-authenticated",
		"https://localhost:8220",
		fleetserver.APIKey("V2R2NVlKUUJtbFRXZFZHaTB5c1U6aFpqdU1udWpUajZBR1FPUUNRRGdWZw=="),
		fleetserver.TLSSkipVerify(),
	)

	status, err := client.Status(t.Context())
	require.NoError(t, err)

	assert.Equal(t, status.Name, "fleet-server")
	assert.Equal(t, status.Status, "HEALTHY")
	assert.NotEmpty(t, status.Version.Number)
}

func TestStatusUnauthenticated(t *testing.T) {
	client := fleetservertest.NewClient(t,
		"./testdata/status-unauthenticated",
		"https://localhost:8220",
		fleetserver.TLSSkipVerify(),
	)

	status, err := client.Status(t.Context())
	require.NoError(t, err)

	assert.Equal(t, status.Name, "fleet-server")
	assert.Equal(t, status.Status, "HEALTHY")
}
