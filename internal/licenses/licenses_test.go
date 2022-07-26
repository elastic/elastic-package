// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package licenses

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteText(t *testing.T) {
	cases := []struct {
		license  string
		expected []byte
		fail     bool
	}{
		{
			license:  Apache20,
			expected: apache20text,
		},
		{
			license:  Elastic20,
			expected: elastic20text,
		},
		{
			license: "not-existing-license",
			fail:    true,
		},
	}

	for _, c := range cases {
		var w bytes.Buffer
		err := WriteText(c.license, &w)
		if c.fail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, string(c.expected), w.String())
		}
	}
}
