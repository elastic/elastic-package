// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var tests = []struct {
	version string
	variant string
}{
	{"", "default"},
	{"7", "default"},
	{"7.0.0", "default"},
	{"7.14.99-SNAPSHOT", "default"},
	{"8", "80"},
	{"8-0", "80"},
	{"8.0.0-alpha", "80"},
	{"8.0.0", "80"},
	{"8.0.33", "80"},
	{"8.0.33-beta", "80"},
	{"8.1-0", "80"},
	{"8.1", "80"},
	{"8.1-alpha", "80"},
	{"8.1.0-alpha", "80"},
	{"8.1.0", "80"},
	{"8.1.58", "80"},
	{"8.1.99-beta", "80"},
	{"8.1.999-SNAPSHOT", "80"},
	{"8.2-0", "86"},
	{"8.2", "86"},
	{"8.2.0-alpha", "86"},
	{"8.2.0", "86"},
	{"8.2.58", "86"},
	{"8.2.99-gamma", "86"},
	{"8.2.777-SNAPSHOT+arm64", "86"},
	{"8.5", "86"},
	{"8.6.1", "86"},
	{"8.7.0", "8x"},
	{"8.7.0-SNAPSHOT", "8x"},
	{"8.7.1-SNAPSHOT", "8x"},
	{"9", "default"},
}

func TestSelectStackVersion(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			selected := selectStackVersion(tt.version)
			assert.Equal(t, tt.variant, selected)
		})
	}
}
