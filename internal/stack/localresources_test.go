// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"testing"
)

func TestEsHostWithPort(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{"host without port", "https://hostname", "https://hostname:443"},
		{"host with port", "https://hostname:443", "https://hostname:443"},
		{"host with differernt port", "https://hostname:9200", "https://hostname:9200"},
		{"ipv6 host", "http://[2001:db8:1f70::999:de8:7648:6e8]:100/", "http://[2001:db8:1f70::999:de8:7648:6e8]:100/"},
		{"ipv6 host without port", "http://[2001:db8:1f70::999:de8:7648:6e8]", "http://[2001:db8:1f70::999:de8:7648:6e8]:443"},
		{"host with path", "https://hostname/xyz", "https://hostname:443/xyz"},
		{"ipv6 host with path", "https://[::1]/xyz", "https://[::1]:443/xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := esHostWithPort(tt.host); got != tt.want {
				t.Errorf("esHostWithPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDockerInternalHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{"host without port", "https://hostname", "https://hostname"},
		{"host with port", "https://hostname:443", "https://hostname:443"},
		{"localhost without port", "https://localhost", "https://host.docker.internal"},
		{"localhost wit port", "https://localhost:443", "https://host.docker.internal:443"},
		{"host with path", "https://hostname/abc", "https://hostname/abc"},
		{"host with port and path", "https://hostname:443/abx", "https://hostname:443/abx"},
		{"localhost with path", "https://localhost/abc", "https://host.docker.internal/abc"},
		{"localhost with port and path", "https://localhost:443/abc", "https://host.docker.internal:443/abc"},
		{"ip with port and path", "http://127.0.1.1:443/abc", "http://host.docker.internal:443/abc"},
		{"ipv6 with port and path", "http://[::1]:443/abc", "http://host.docker.internal:443/abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DockerInternalHost(tt.host); got != tt.want {
				t.Errorf("dockerInternalHost() = %v, want %v", got, tt.want)
			}
		})
	}
}
