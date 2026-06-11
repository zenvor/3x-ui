package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

func TestAccessLogKeepsRecentEntries(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 1)
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	svc := &AccessLogService{now: func() time.Time {
		now = now.Add(time.Second)
		return now
	}}

	for i := 0; i < 105; i++ {
		if err := svc.Record(AccessLogInput{
			SubscriptionId: sub.Id,
			Endpoint:       AccessEndpointFull,
			Ip:             fmt.Sprintf("10.0.0.%d", i),
			UserAgent:      "mihomo",
			StatusCode:     200,
			Result:         AccessResultSuccess,
		}); err != nil {
			t.Fatalf("record log %d: %v", i, err)
		}
	}

	logs, err := svc.List(sub.Id)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != accessLogRetention {
		t.Fatalf("logs len = %d, want %d", len(logs), accessLogRetention)
	}
	if logs[0].Ip != "10.0.0.104" {
		t.Fatalf("first log ip = %s, want latest", logs[0].Ip)
	}
	if logs[len(logs)-1].Ip != "10.0.0.5" {
		t.Fatalf("last log ip = %s, want oldest retained", logs[len(logs)-1].Ip)
	}
}

func TestAccessLogNormalizesUserAgent(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 1)
	longUA := strings.Repeat("a", accessLogUserAgentMaxRunes+20) + "\n\tbad"

	if err := NewAccessLogService().Record(AccessLogInput{
		SubscriptionId: sub.Id,
		Endpoint:       AccessEndpointFull,
		Ip:             "1.1.1.1",
		UserAgent:      longUA,
		StatusCode:     200,
		Result:         AccessResultSuccess,
	}); err != nil {
		t.Fatalf("record log: %v", err)
	}

	logs, err := NewAccessLogService().List(sub.Id)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len([]rune(logs[0].UserAgent)) != accessLogUserAgentMaxRunes {
		t.Fatalf("user agent len = %d, want %d", len([]rune(logs[0].UserAgent)), accessLogUserAgentMaxRunes)
	}
	if strings.ContainsAny(logs[0].UserAgent, "\n\t") {
		t.Fatalf("user agent contains control characters: %q", logs[0].UserAgent)
	}
}

func TestAccessLogListRecentIncludesSubscriptionRemark(t *testing.T) {
	setupTestDB(t)
	first := seedSubscription(t, 1)
	second := seedSubscription(t, 2)
	first.Remark = "first sub"
	second.Remark = "second sub"
	if err := database.GetDB().Model(first).Update("remark", first.Remark).Error; err != nil {
		t.Fatalf("update first remark: %v", err)
	}
	if err := database.GetDB().Model(second).Update("remark", second.Remark).Error; err != nil {
		t.Fatalf("update second remark: %v", err)
	}

	now := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	svc := &AccessLogService{now: func() time.Time {
		now = now.Add(time.Second)
		return now
	}}
	for _, sub := range []struct {
		id int
		ip string
	}{
		{first.Id, "10.0.0.1"},
		{second.Id, "10.0.0.2"},
		{first.Id, "10.0.0.3"},
	} {
		if err := svc.Record(AccessLogInput{
			SubscriptionId: sub.id,
			Endpoint:       AccessEndpointFull,
			Ip:             sub.ip,
			UserAgent:      "mihomo",
			StatusCode:     200,
			Result:         AccessResultSuccess,
		}); err != nil {
			t.Fatalf("record log: %v", err)
		}
	}

	items, err := svc.ListRecent(2)
	if err != nil {
		t.Fatalf("list recent logs: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if items[0].Ip != "10.0.0.3" || items[0].SubscriptionRemark != "first sub" {
		t.Fatalf("latest item unexpected: %+v", items[0])
	}
	if items[1].Ip != "10.0.0.2" || items[1].SubscriptionRemark != "second sub" {
		t.Fatalf("second item unexpected: %+v", items[1])
	}
}

func TestAccessLogListRecentClampsOversizedLimit(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 1)
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	logs := make([]model.AccessLog, 0, accessLogListMaxLimit+5)
	for i := 0; i < accessLogListMaxLimit+5; i++ {
		logs = append(logs, model.AccessLog{
			SubscriptionId: sub.Id,
			Endpoint:       AccessEndpointFull,
			Ip:             fmt.Sprintf("10.1.0.%d", i),
			UserAgent:      "mihomo",
			StatusCode:     200,
			Result:         AccessResultSuccess,
			AccessedAt:     now.Add(time.Duration(i) * time.Second),
		})
	}
	if err := database.GetDB().Create(&logs).Error; err != nil {
		t.Fatalf("seed logs: %v", err)
	}

	items, err := NewAccessLogService().ListRecent(accessLogListMaxLimit + 100)
	if err != nil {
		t.Fatalf("list recent logs: %v", err)
	}
	if len(items) != accessLogListMaxLimit {
		t.Fatalf("items len = %d, want %d", len(items), accessLogListMaxLimit)
	}
}
