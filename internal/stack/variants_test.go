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
	{"8.0.0-alpha", "80"},
	{"8.0.0", "80"},
	{"8.0.33", "80"},
	{"8.0.33-beta", "80"},
	{"8.1", "80"},
	{"8.1-alpha", "80"},
	{"8.1.0-alpha", "80"},
	{"8.1.0", "80"},
	{"8.1.58", "80"},
	{"8.1.99-beta", "80"},
	{"8.1.999-SNAPSHOT", "80"},
	{"8.2", "8x"},
	{"8.2.0-alpha", "8x"},
	{"8.2.0", "8x"},
	{"8.2.58", "8x"},
	{"8.2.99-gamma", "8x"},
	{"8.2.777-SNAPSHOT+arm64", "8x"},
	{"8.5", "8x"},
	{"9", "default"},
}

func TestSelectStackVersion(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			selected := selectStackVersion(tt.version)
			assert.Equal(t, selected, tt.variant)
		})
	}
}
