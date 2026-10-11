package web

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestStopCronJobsBoundsWaitForBlockedJob(t *testing.T) {
	scheduler := cron.New()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	finished := make(chan struct{})
	_, err := scheduler.AddFunc("@every 1s", func() {
		started <- struct{}{}
		<-release
		close(finished)
	})
	if err != nil {
		t.Fatal(err)
	}
	scheduler.Start()
	t.Cleanup(func() {
		close(release)
		<-scheduler.Stop().Done()
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("cron job did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := stopCronJobs(ctx, scheduler); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stop error = %v, want a bounded deadline", err)
	}
	select {
	case <-finished:
		t.Fatal("test job unexpectedly completed before teardown could continue")
	default:
	}
}

func TestStopCronJobsReturnsWhenJobsComplete(t *testing.T) {
	scheduler := cron.New()
	scheduler.Start()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := stopCronJobs(ctx, scheduler); err != nil {
		t.Fatalf("stop idle scheduler: %v", err)
	}
}
