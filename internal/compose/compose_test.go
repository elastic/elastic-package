// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package compose

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/docker"
)

func TestIntOrStringYaml(t *testing.T) {
	cases := []struct {
		yaml     string
		expected int
	}{
		{`"9200"`, 9200},
		{`'9200'`, 9200},
		{`9200`, 9200},
	}

	for _, c := range cases {
		t.Run(c.yaml, func(t *testing.T) {
			var n intOrStringYaml
			err := yaml.Unmarshal([]byte(c.yaml), &n)
			require.NoError(t, err)
			assert.Equal(t, c.expected, int(n))
		})
	}
}

func TestGetVersionFromDockerImage(t *testing.T) {
	cases := []struct {
		dockerImage string
		expected    string
	}{
		{"docker.test/test:1.42.0", "1.42.0"},
		{"docker.test/test", "latest"},
	}

	for _, c := range cases {
		t.Run(c.dockerImage, func(t *testing.T) {
			version := getVersionFromDockerImage(c.dockerImage)
			assert.Equal(t, c.expected, version)
		})
	}
}

func TestNewServiceStatus(t *testing.T) {
	cases := []struct {
		name        string
		description docker.ContainerDescription
		expected    ServiceStatus
	}{
		{
			name: "commonService",
			description: docker.ContainerDescription{
				Config: struct {
					Image  string
					Labels map[string]string
				}{
					Image:  "docker.test:1.42.0",
					Labels: map[string]string{"com.docker.compose.service": "myservice", "foo": "bar"},
				},
				ID:   "123456789ab",
				Name: "project-my-service",
				State: struct {
					Status   string
					ExitCode int
					Health   *struct {
						Status string
						Log    []struct {
							Start    time.Time
							ExitCode int
							Output   string
						}
					}
				}{
					Status:   "running",
					ExitCode: 0,
					Health: &struct {
						Status string
						Log    []struct {
							Start    time.Time
							ExitCode int
							Output   string
						}
					}{
						Status: "healthy",
					},
				},
			},
			expected: ServiceStatus{
				ID:      "123456789ab",
				Name:    "myservice",
				Status:  "running (healthy)",
				Version: "1.42.0",
			},
		},
		{
			name: "exitedService",
			description: docker.ContainerDescription{
				Config: struct {
					Image  string
					Labels map[string]string
				}{
					Image:  "docker.test:1.42.0",
					Labels: map[string]string{"com.docker.compose.service": "myservice", "foo": "bar"},
				},
				ID:   "123456789ab",
				Name: "project-my-service",
				State: struct {
					Status   string
					ExitCode int
					Health   *struct {
						Status string
						Log    []struct {
							Start    time.Time
							ExitCode int
							Output   string
						}
					}
				}{
					Status:   "exited",
					ExitCode: 128,
					Health:   nil,
				},
			},
			expected: ServiceStatus{
				ID:      "123456789ab",
				Name:    "myservice",
				Status:  "exited",
				Version: "1.42.0",
			},
		},
		{
			name: "startingService",
			description: docker.ContainerDescription{
				Config: struct {
					Image  string
					Labels map[string]string
				}{
					Image:  "docker.test:1.42.0",
					Labels: map[string]string{"com.docker.compose.service": "myservice", "foo": "bar"},
				},
				ID:   "123456789ab",
				Name: "project-my-service",
				State: struct {
					Status   string
					ExitCode int
					Health   *struct {
						Status string
						Log    []struct {
							Start    time.Time
							ExitCode int
							Output   string
						}
					}
				}{
					Status:   "running",
					ExitCode: 0,
					Health: &struct {
						Status string
						Log    []struct {
							Start    time.Time
							ExitCode int
							Output   string
						}
					}{
						Status: "starting",
					},
				},
			},
			expected: ServiceStatus{
				ID:      "123456789ab",
				Name:    "myservice",
				Status:  "running (starting)",
				Version: "1.42.0",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			serviceStatus, err := newServiceStatus(&c.description)
			require.NoError(t, err)
			assert.Equal(t, &c.expected, serviceStatus)
		})
	}
}
