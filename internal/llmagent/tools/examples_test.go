// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetExampleContent_ErrorCases(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		_, err := GetExampleContent("", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "example name is required")
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := GetExampleContent("non_existent.md", "")
		assert.Error(t, err)
	})

	t.Run("non-existent section", func(t *testing.T) {
		_, err := GetExampleContent("fortinet_fortigate.md", "Non Existent")
		assert.Error(t, err)
	})
}
