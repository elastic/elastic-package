// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package jenkins

import (
	"context"
	"log"
	"time"
)

type retryableFunction func(context.Context) error

func retry(f retryableFunction, retries int, delay time.Duration) retryableFunction {
	return func(ctx context.Context) error {
		for r := 0; ; r++ {
			err := f(ctx)
			if err == nil || r >= retries {
				// Return when there is no error or the maximum amount
				// of retries is reached.
				return err
			}

			log.Printf("Function failed, retrying in %v", delay)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
}
