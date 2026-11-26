// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/stack"
)

func TestPrintStatus(t *testing.T) {
	agentLabels := &docker.ConfigLabels{BuildDate: "2026-10-01T03:11:00Z", VCSRef: "f971781446ab46d1aa46ad8ea7cb08c4dd3ef9ee"}
	services := []stack.ServiceStatus{
		{Name: "elastic-agent", Version: "9.5.5", Status: "running (healthy)", Labels: agentLabels},
		{Name: "elasticsearch", Version: "9.5.5", Status: "running (healthy)"},
		{Name: "fleet-server", Version: "9.5.5", Status: "running (healthy)", Labels: agentLabels},
		{Name: "kibana", Version: "9.5.5", Status: "running (starting)"},
		{Name: "package-registry", Version: "latest", Status: "exited (1)"},
	}

	cases := []struct {
		title    string
		urls     map[string]string
		expected string
	}{
		{
			title:    "without URLs",
			expected: "./testdata/stack-status",
		},
		{
			title: "with URLs",
			urls: map[string]string{
				"elasticsearch": "https://127.0.0.1:35813",
				"fleet-server":  "https://127.0.0.1:41873",
				// Long URLs should not be wrapped.
				"kibana": "https://a-profile-with-a-long-name.localhost:35007",
			},
			expected: "./testdata/stack-status-urls",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			servicesStatus := make([]stack.ServiceStatus, len(services))
			for i, service := range services {
				service.URL = c.urls[service.Name]
				servicesStatus[i] = service
			}

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)
			err := printStatus(cmd, servicesStatus)
			require.NoError(t, err)

			assertOutputWithFile(t, c.expected, buf.String())
		})
	}
}

func TestValidateServicesFlag(t *testing.T) {
	t.Run("a non available service returns error", func(t *testing.T) {
		err := validateServicesFlag([]string{"non-existing-service"})
		require.Error(t, err)
	})

	t.Run("a non available service in a valid stack returns error", func(t *testing.T) {
		err := validateServicesFlag([]string{"elasticsearch", "non-existing-service"})
		require.Error(t, err)
	})

	t.Run("not possible to start a service twice", func(t *testing.T) {
		err := validateServicesFlag([]string{"elasticsearch", "elasticsearch"})
		require.Error(t, err)
	})

	availableStackServicesTest := []struct {
		services []string
	}{
		{services: []string{"elastic-agent"}},
		{services: []string{"elastic-agent", "elasticsearch"}},
		{services: []string{"elasticsearch"}},
		{services: []string{"kibana"}},
		{services: []string{"package-registry"}},
		{services: []string{"fleet-server"}},
		{services: []string{"elasticsearch", "fleet-server"}},
	}

	for _, srv := range availableStackServicesTest {
		t.Run(fmt.Sprintf("%v are available as a stack service", srv.services), func(t *testing.T) {
			err := validateServicesFlag(srv.services)
			require.Nil(t, err)
		})
	}

}
