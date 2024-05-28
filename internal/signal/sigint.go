// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package signal

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
)

// Enable returns a context configured to be cancelled if an interruption signal
// is received.
// Returned context can be cancelled explicitly with the returned function.
func Enable(ctx context.Context, logger *slog.Logger) (notifyCtx context.Context, stop func()) {
	notifyCtx, stopNotify := signal.NotifyContext(ctx, os.Interrupt)
	stopLogger := context.AfterFunc(notifyCtx, func() {
		logger.Info("Signal caught!")
	})

	return notifyCtx, func() {
		stopLogger()
		stopNotify()
	}
}
