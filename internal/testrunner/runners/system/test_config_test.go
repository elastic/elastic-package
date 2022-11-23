// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetExpandedConfigValue(t *testing.T) {
	const configData = `
vars:
  jmx.mappings: |
    - mbean: 'java.lang:type=Runtime'
      attributes:
        - attr: Uptime
          field: uptime
`

	config, err := parseConfig("test", []byte(configData))
	require.NoError(t, err)

	_, found := getConfigValue(config.Vars, "jmx.mappings")
	assert.True(t, found)
}
