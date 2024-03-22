// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareJsonNumber(t *testing.T) {
	cases := []struct {
		want  json.Number
		got   json.Number
		equal bool
	}{
		{"0", "0", true},
		{"0.0", "0", true},
		{"0", "0.0", true},
		{"42", "42", true},
		{"42.0", "42", true},
		{"42", "42.0", true},
		{"0.42", "0.42", true},
		{"-10", "-10", true},
		{"-10.0", "-10", true},
		{"6920071768563516000", "6920071768563516000", true},
		{"6920071768563516847", "6920071768563516847", true},
		{"1624617166.182", "1.624617166182E9", true},

		{"0", "1", false},
		{"0.1", "0", false},
		{"6920071768563516000", "6920071768563516847", false},
		{"1624617166.182", "1.624617166181E9", false},
	}

	for _, c := range cases {
		t.Run(c.want.String()+" == "+c.got.String(), func(t *testing.T) {
			equal := compareJsonNumbers(c.want, c.got)
			assert.Equal(t, c.equal, equal)
		})
	}
}
