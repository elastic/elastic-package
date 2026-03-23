// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package wait

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPollBudget(t *testing.T) {
	cases := []struct {
		name   string
		window time.Duration
		period time.Duration
		want   int
	}{
		{"zero window", 0, time.Second, 1},
		{"negative window", -time.Second, time.Second, 1},
		{"zero period", time.Second, 0, 1},
		{"negative period", time.Second, -time.Millisecond, 1},
		{"exact multiple", 2 * time.Second, time.Second, 2},
		{"ceil partial", 1500 * time.Millisecond, time.Second, 2},
		{"sub-period window still one poll", time.Millisecond, time.Second, 1},
		{"milliseconds", 500 * time.Millisecond, 100 * time.Millisecond, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, PollBudget(c.window, c.period))
		})
	}
}
