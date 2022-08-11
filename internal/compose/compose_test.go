// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
