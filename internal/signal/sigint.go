// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package signal

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
)

var ch chan os.Signal

// Enable function enables signal notifications.
func Enable() {
	ch = make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}

// SIGINT function returns true if ctrl+c was pressed
func SIGINT() bool {
	select {
	case <-ch:
		logger.Info("Signal caught!")
		return true
	default:
		return false
	}
}

// Sleep is the equivalent of time.Sleep with the exception
// that is will end the sleep if ctrl+c is pressed.
func Sleep(d time.Duration) {
	timer := time.NewTimer(d)
	select {
	case <-ch:
		logger.Info("Signal caught!")
		timer.Stop()
	case <-timer.C:
	}
}
