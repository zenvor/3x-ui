package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

func TestUsageRecordCompletedIncrementsStats(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 1)
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	svc := &SubscriptionUsageService{now: func() time.Time { return now }}

	if err := svc.RecordCompleted(sub.Id, "1.1.1.1", "mihomo"); err != nil {
		t.Fatalf("record first completion: %v", err)
	}
	now = now.Add(time.Second)
	if err := svc.RecordCompleted(sub.Id, "2.2.2.2", "mihomo/2"); err != nil {
		t.Fatalf("record second completion: %v", err)
	}

	var stats model.SubscriptionStats
	if err := database.GetDB().First(&stats, "subscription_id = ?", sub.Id).Error; err != nil {
		t.Fatalf("load stats: %v", err)
	}
	if stats.CompletedCount != 2 {
		t.Fatalf("completed count = %d, want 2", stats.CompletedCount)
	}
	if stats.LastCompletedIp != "2.2.2.2" {
		t.Fatalf("last completed ip = %q, want 2.2.2.2", stats.LastCompletedIp)
	}
	if stats.LastCompletedUserAgent != "mihomo/2" {
		t.Fatalf("last completed user agent = %q, want mihomo/2", stats.LastCompletedUserAgent)
	}
	if stats.LastCompletedAt == nil || !stats.LastCompletedAt.Equal(now) {
		t.Fatalf("last completed at = %v, want %v", stats.LastCompletedAt, now)
	}
}

func TestUsageRecordCompletedWithoutDBIsNoop(t *testing.T) {
	if err := database.Reset(); err != nil {
		t.Fatalf("reset db: %v", err)
	}

	svc := NewSubscriptionUsageService()
	if err := svc.RecordCompleted(1, "1.1.1.1", "mihomo"); err != nil {
		t.Fatalf("record without db should not fail: %v", err)
	}
}
