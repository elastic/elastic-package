// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSizeMarshallJSON(t *testing.T) {
	cases := []struct {
		fileSize ByteSize
		expected string
	}{
		{ByteSize(0), `"0B"`},
		{ByteSize(1024), `"1KB"`},
		{ByteSize(1025), `"1025B"`},
		{5 * MegaByte, `"5MB"`},
		{5 * GigaByte, `"5GB"`},
	}

	for _, c := range cases {
		t.Run(c.expected, func(t *testing.T) {
			d, err := json.Marshal(c.fileSize)
			require.NoError(t, err)
			assert.Equal(t, c.expected, string(d))
		})
	}
}

func TestFileSizeMarshallYAML(t *testing.T) {
	cases := []struct {
		fileSize ByteSize
		expected string
	}{
		{ByteSize(0), "0B\n"},
		{ByteSize(1024), "1KB\n"},
		{ByteSize(1025), "1025B\n"},
		{5 * MegaByte, "5MB\n"},
		{5 * GigaByte, "5GB\n"},
	}

	for _, c := range cases {
		t.Run(c.expected, func(t *testing.T) {
			d, err := yaml.Marshal(c.fileSize)
			require.NoError(t, err)
			assert.Equal(t, c.expected, string(d))
		})
	}
}

func TestFileSizeUnmarshal(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		testFileSizeUnmarshalFormat(t, json.Unmarshal)
	})
	t.Run("yaml", func(t *testing.T) {
		testFileSizeUnmarshalFormat(t, yaml.Unmarshal)
	})
}

func testFileSizeUnmarshalFormat(t *testing.T, unmarshaler func([]byte, interface{}) error) {
	cases := []struct {
		json     string
		expected ByteSize
		valid    bool
	}{
		{"0", 0, true},
		{"1024", 1024 * Byte, true},
		{`"1024"`, 1024 * Byte, true},
		{`"1024B"`, 1024 * Byte, true},
		{`"10MB"`, 10 * MegaByte, true},
		{`"40GB"`, 40 * GigaByte, true},
		{`"56.21GB"`, approxFloat(56.21, GigaByte), true},
		{`"2KB"`, 2 * KiloByte, true},
		{`"KB"`, 0, false},
		{`"1s"`, 0, false},
		{`""`, 0, false},
		{`"B"`, 0, false},
		{`"-200MB"`, 0, false},
		{`"-1"`, 0, false},
		{`"10000000000000000000MB"`, 0, false},
	}

	for _, c := range cases {
		t.Run(c.json, func(t *testing.T) {
			var found ByteSize
			err := unmarshaler([]byte(c.json), &found)
			if c.valid {
				require.NoError(t, err)
				assert.Equal(t, c.expected, found)
			} else {
				require.Error(t, err)
			}
		})
	}
}
