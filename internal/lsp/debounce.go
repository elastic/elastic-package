// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"sync"
	"time"
)

const debounceDelay = 500 * time.Millisecond

// Debouncer coalesces rapid validation triggers per package root.
type Debouncer struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
}

// NewDebouncer creates a new Debouncer.
func NewDebouncer() *Debouncer {
	return &Debouncer{
		timers: make(map[string]*time.Timer),
	}
}

// Trigger schedules fn to run after the debounce delay. If Trigger is called
// again for the same key before the delay elapses, the previous call is
// cancelled and the timer resets.
func (d *Debouncer) Trigger(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if t, ok := d.timers[key]; ok {
		t.Stop()
	}

	d.timers[key] = time.AfterFunc(debounceDelay, fn)
}

// Shutdown stops all pending timers.
func (d *Debouncer) Shutdown() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, t := range d.timers {
		t.Stop()
		delete(d.timers, key)
	}
}
