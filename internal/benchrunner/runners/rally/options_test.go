// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package rally

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewOptions(t *testing.T) {
	t.Parallel()

	t.Run("defaults_are_zero", func(t *testing.T) {
		t.Parallel()
		opts := NewOptions()
		assert.Zero(t, opts.DeferCleanup)
		assert.Zero(t, opts.MetricsInterval)
		assert.Empty(t, opts.BenchName)
		assert.Empty(t, opts.Variant)
		assert.False(t, opts.ReindexData)
	})

	t.Run("with_defer_cleanup", func(t *testing.T) {
		t.Parallel()
		opts := NewOptions(WithDeferCleanup(30 * time.Minute))
		assert.Equal(t, 30*time.Minute, opts.DeferCleanup)
	})

	t.Run("with_metrics_interval", func(t *testing.T) {
		t.Parallel()
		opts := NewOptions(WithMetricsInterval(5 * time.Second))
		assert.Equal(t, 5*time.Second, opts.MetricsInterval)
	})

	t.Run("combined_options", func(t *testing.T) {
		t.Parallel()
		opts := NewOptions(
			WithBenchmarkName("my-bench"),
			WithVariant("default"),
			WithDeferCleanup(10*time.Minute),
			WithMetricsInterval(2*time.Second),
			WithDataReindexing(true),
			WithRallyDryRun(true),
		)
		assert.Equal(t, "my-bench", opts.BenchName)
		assert.Equal(t, "default", opts.Variant)
		assert.Equal(t, 10*time.Minute, opts.DeferCleanup)
		assert.Equal(t, 2*time.Second, opts.MetricsInterval)
		assert.True(t, opts.ReindexData)
		assert.True(t, opts.DryRun)
	})

	t.Run("last_option_wins", func(t *testing.T) {
		t.Parallel()
		opts := NewOptions(
			WithDeferCleanup(5*time.Minute),
			WithDeferCleanup(20*time.Minute),
		)
		assert.Equal(t, 20*time.Minute, opts.DeferCleanup)
	})
}
