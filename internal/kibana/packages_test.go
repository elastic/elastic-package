// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupportsAlertRuleTemplates(t *testing.T) {
	cases := []struct {
		number   string
		snapshot bool
		expected bool
	}{
		{number: "8.19.0", expected: false},
		{number: "9.1.9", expected: false},
		{number: "9.2.0", expected: true},
		{number: "9.2.0", snapshot: true, expected: true},
		{number: "9.6.0", snapshot: true, expected: true},
		{number: "", expected: true},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s snapshot=%t", c.number, c.snapshot), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"version":{"number":%q,"build_snapshot":%t}}`, c.number, c.snapshot)
			}))
			defer server.Close()

			client, err := NewClient(Address(server.URL))
			require.NoError(t, err)
			assert.Equal(t, c.expected, client.SupportsAlertRuleTemplates())
		})
	}
}
