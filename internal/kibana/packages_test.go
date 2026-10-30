// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetLogsDBColumnar(t *testing.T) {
	cases := []struct {
		title        string
		enabled      bool
		status       int
		expectedBody string
		expectedErr  string
	}{
		{
			title:        "turn on",
			enabled:      true,
			status:       http.StatusOK,
			expectedBody: `{"logsdb_columnar":true}`,
		},
		{
			title:        "turn off",
			enabled:      false,
			status:       http.StatusOK,
			expectedBody: `{"logsdb_columnar":false}`,
		},
		{
			title:        "refused by Fleet",
			enabled:      true,
			status:       http.StatusBadRequest,
			expectedBody: `{"logsdb_columnar":true}`,
			expectedErr:  "could not set logsdb_columnar to true for package nginx; API status code = 400",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			var method, path, body string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path = r.Method, r.URL.Path
				b, _ := io.ReadAll(r.Body)
				body = string(b)
				w.WriteHeader(c.status)
				fmt.Fprint(w, `{}`)
			}))
			t.Cleanup(server.Close)

			client := newTestClient(t, server.URL)
			err := client.SetLogsDBColumnar(t.Context(), "nginx", c.enabled)
			if c.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.expectedErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, http.MethodPut, method)
			assert.Equal(t, "/api/fleet/epm/packages/nginx", path)
			assert.JSONEq(t, c.expectedBody, body)
		})
	}
}

func TestGetPackageLogsDBColumnarEnabled(t *testing.T) {
	cases := []struct {
		title    string
		response string
		expected bool
	}{
		{
			title:    "enabled",
			response: `{"item":{"name":"nginx","installationInfo":{"logsdb_columnar_enabled":true}}}`,
			expected: true,
		},
		{
			title:    "disabled",
			response: `{"item":{"name":"nginx","installationInfo":{"logsdb_columnar_enabled":false}}}`,
			expected: false,
		},
		{
			title:    "not returned by Kibana",
			response: `{"item":{"name":"nginx","installationInfo":{}}}`,
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, c.response)
			}))
			t.Cleanup(server.Close)

			client := newTestClient(t, server.URL)
			pkg, err := client.GetPackage(t.Context(), "nginx")
			require.NoError(t, err)
			require.NotNil(t, pkg.InstallationInfo)
			assert.Equal(t, c.expected, pkg.InstallationInfo.LogsDBColumnarEnabled)
		})
	}
}

func newTestClient(t *testing.T, address string) *Client {
	version := func(c *Client) {
		c.versionInfo = VersionInfo{Number: "9.6.0"}
		c.semver = semver.MustParse(c.versionInfo.Number)
	}
	client, err := NewClient(version, Address(address), RetryMax(0))
	require.NoError(t, err)
	return client
}
