package web

import (
	"context"

	"github.com/robfig/cron/v3"
)

// Stop scheduling immediately, then wait only within the shutdown budget.
// A job stuck in an external service must not prevent the remaining teardown.
func stopCronJobs(ctx context.Context, scheduler *cron.Cron) error {
	stopped := scheduler.Stop()
	select {
	case <-stopped.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
