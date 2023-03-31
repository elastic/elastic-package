package jenkins

import (
	"context"
	"log"
	"time"
)

type buildFunction func(context.Context, string, map[string]string) (int64, error)

func retry(f buildFunction, retries int, delay time.Duration) buildFunction {
	return func(ctx context.Context, jobName string, params map[string]string) (int64, error) {
		for r := 0; ; r++ {
			response, err := f(ctx, jobName, params)
			if err == nil || r >= retries {
				// Return when there is no error or the maximum amount
				// of retries is reached.
				return response, err
			}

			log.Printf("Function call failed, retrying in %v", delay)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
	}
}
