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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := esHostWithPort(tt.host); got != tt.want {
				t.Errorf("esHostWithPort() = %v, want %v", got, tt.want)
			}
		})
	}
}
