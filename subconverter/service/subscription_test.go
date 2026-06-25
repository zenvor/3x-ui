package service

import (
	"path/filepath"
	"strconv"
	"testing"

	xdatabase "github.com/mhsanaei/3x-ui/v3/internal/database"
	xmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

// setupTestDB points InitDB at a fresh per-test temp directory so the
// package-level GORM handle in subconverter/database is bound to a clean
// SQLite file with all migrations applied.
func setupTestDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	if err := xdatabase.CloseDB(); err != nil {
		t.Fatalf("close main db: %v", err)
	}
	if err := xdatabase.InitDB(filepath.Join(tmpDir, "x-ui.db")); err != nil {
		t.Fatalf("init main db: %v", err)
	}
	if err := database.Reset(); err != nil {
		t.Fatalf("reset db: %v", err)
	}
	if err := database.InitDB(); err != nil {
		t.Fatalf("init db: %v", err)
	}
	seedSubscriptionTestInbounds(t, map[int][]xmodel.Client{
		1:  {testClient("uuid-1", "alice@x")},
		2:  {testClient("uuid-2", "alice@x")},
		7:  {testClient("uuid-7", "alice@x")},
		10: {testClient("uuid-10", "alice@x")},
	})
	t.Cleanup(func() {
		_ = database.Reset()
		_ = xdatabase.CloseDB()
	})
}

func seedSubscriptionTestInbounds(t *testing.T, clientsByID map[int][]xmodel.Client) {
	t.Helper()
	for id, clients := range clientsByID {
		inbound := vlessInboundWithClients(id, clients, realityStream())
		if id == 10 {
			inbound.StreamSettings = xhttpNoneStream()
		}
		if err := xdatabase.GetDB().Create(inbound).Error; err != nil {
			t.Fatalf("seed inbound %d: %v", id, err)
		}
	}
}

func vlessInboundWithClients(id int, clients []xmodel.Client, streamSettings string) *xmodel.Inbound {
	settings := `{"clients":[`
	for i, client := range clients {
		if i > 0 {
			settings += `,`
		}
		settings += `{"id":"` + client.ID + `","email":"` + client.Email + `","enable":` + boolJSON(client.Enable) + `}`
	}
	settings += `]}`
	return &xmodel.Inbound{
		Id:             id,
		Remark:         "inbound",
		Tag:            "subconverter-test-inbound-" + strconv.Itoa(id),
		Enable:         true,
		Port:           40000 + id,
		Protocol:       xmodel.VLESS,
		StreamSettings: streamSettings,
		Settings:       settings,
	}
}

func testClient(id, email string) xmodel.Client {
	return xmodel.Client{ID: id, Email: email, Enable: true}
}

func boolJSON(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func TestSubscriptionCRUD(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	enabled := true
	created, err := svc.Create(SubscriptionInput{
		Remark:  "family",
		MaxIps:  3,
		Enabled: &enabled,
		Inbounds: []InboundInput{
			{InboundId: 1, ClientEmail: ""},
			{InboundId: 2, ClientEmail: "alice@x"},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Id == 0 {
		t.Fatal("created subscription has zero id")
	}
	if len(created.Token) != 32 {
		t.Fatalf("token length = %d, want 32", len(created.Token))
	}
	if len(created.Inbounds) != 2 {
		t.Fatalf("created inbounds len = %d, want 2", len(created.Inbounds))
	}
	ipSvc := NewIPBindingService()
	if err := ipSvc.Enforce(created.Id, created.MaxIps, "1.1.1.1"); err != nil {
		t.Fatalf("bind first ip: %v", err)
	}
	if err := ipSvc.Enforce(created.Id, created.MaxIps, "2.2.2.2"); err != nil {
		t.Fatalf("bind second ip: %v", err)
	}

	subs, err := svc.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 1 || subs[0].Token != created.Token {
		t.Fatalf("unexpected list result: %+v", subs)
	}
	if subs[0].BoundIpCount != 2 {
		t.Fatalf("list bound ip count = %d, want 2", subs[0].BoundIpCount)
	}

	got, err := svc.Get(created.Id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Remark != "family" || got.MaxIps != 3 || !got.Enabled {
		t.Fatalf("get returned unexpected data: %+v", got)
	}
	if len(got.Inbounds) != 2 {
		t.Fatalf("get inbounds len = %d, want 2", len(got.Inbounds))
	}
	if got.BoundIpCount != 2 {
		t.Fatalf("get bound ip count = %d, want 2", got.BoundIpCount)
	}
	if err := NewAccessLogService().Record(AccessLogInput{
		SubscriptionId: created.Id,
		Endpoint:       AccessEndpointFull,
		Ip:             "1.1.1.1",
		UserAgent:      "mihomo",
		StatusCode:     200,
		Result:         AccessResultSuccess,
	}); err != nil {
		t.Fatalf("record access log before detail: %v", err)
	}
	detail, err := svc.GetDetail(created.Id)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if detail.Id != created.Id || detail.BoundIpCount != 2 {
		t.Fatalf("get detail returned unexpected subscription: %+v", detail.Subscription)
	}
	if len(detail.BoundIps) != 2 {
		t.Fatalf("detail bound ips len = %d, want 2", len(detail.BoundIps))
	}
	if len(detail.AccessLogs) != 1 || detail.AccessLogs[0].Result != AccessResultSuccess {
		t.Fatalf("detail access logs unexpected: %+v", detail.AccessLogs)
	}

	disabled := false
	updated, err := svc.Update(created.Id, SubscriptionInput{
		Remark:  "team",
		MaxIps:  5,
		Enabled: &disabled,
		Inbounds: []InboundInput{
			{InboundId: 7, ClientEmail: ""},
		},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Remark != "team" || updated.MaxIps != 5 || updated.Enabled {
		t.Fatalf("update did not persist mutable fields: %+v", updated)
	}
	if len(updated.Inbounds) != 1 || updated.Inbounds[0].InboundId != 7 {
		t.Fatalf("update did not replace inbound list: %+v", updated.Inbounds)
	}
	if updated.Token != created.Token {
		t.Fatal("update should not change token")
	}

	if err := NewSubscriptionUsageService().RecordCompleted(created.Id, "1.1.1.1", "mihomo"); err != nil {
		t.Fatalf("record usage before delete: %v", err)
	}
	if err := svc.Delete(created.Id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	subs, _ = svc.List()
	if len(subs) != 0 {
		t.Fatalf("expected empty list after delete, got %d", len(subs))
	}
	var statsCount int64
	if err := database.GetDB().Model(&model.SubscriptionStats{}).
		Where("subscription_id = ?", created.Id).
		Count(&statsCount).Error; err != nil {
		t.Fatalf("count stats after delete: %v", err)
	}
	if statsCount != 0 {
		t.Fatalf("expected stats to be deleted, got %d", statsCount)
	}
}

func TestCreateRequiresInbounds(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	if _, err := svc.Create(SubscriptionInput{Remark: "x"}); err == nil {
		t.Fatal("expected error when input has no inbounds")
	}
}

func TestSubscriptionKeepsLegacyEmptyClientEmailReadable(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()
	legacy := &model.Subscription{
		Token:   "legacy-empty-client-email",
		Remark:  "legacy",
		Enabled: true,
	}
	if err := database.GetDB().Create(legacy).Error; err != nil {
		t.Fatalf("create legacy sub: %v", err)
	}
	if err := database.GetDB().Create(&model.SubscriptionInbound{
		SubscriptionId: legacy.Id,
		InboundId:      1,
		ClientEmail:    "",
	}).Error; err != nil {
		t.Fatalf("create legacy inbound: %v", err)
	}

	got, err := svc.Get(legacy.Id)
	if err != nil {
		t.Fatalf("get legacy: %v", err)
	}
	if len(got.Inbounds) != 1 {
		t.Fatalf("legacy inbounds len = %d, want 1", len(got.Inbounds))
	}
	if got.Inbounds[0].ClientEmail != "" {
		t.Fatalf("legacy clientEmail = %q, want empty", got.Inbounds[0].ClientEmail)
	}
}

func TestSubscriptionPersistsCDNTLSOverride(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()
	enabled := true

	created, err := svc.Create(SubscriptionInput{
		Remark:  "cdn",
		MaxIps:  1,
		Enabled: &enabled,
		Inbounds: []InboundInput{{
			InboundId:     10,
			CdnTLS:        true,
			CdnServer:     " edge.example.com ",
			CdnServerName: "",
			CdnXHTTPHost:  "",
		}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(created.Inbounds) != 1 {
		t.Fatalf("inbounds len = %d, want 1", len(created.Inbounds))
	}
	got := created.Inbounds[0]
	if !got.CdnTLS || got.CdnServer != "edge.example.com" || got.CdnPort != 443 {
		t.Fatalf("cdn endpoint not normalized: %+v", got)
	}
	if got.CdnServerName != "edge.example.com" || got.CdnXHTTPHost != "edge.example.com" {
		t.Fatalf("cdn sni/host not defaulted: %+v", got)
	}
	if got.CdnClientFp != "chrome" {
		t.Fatalf("cdn client defaults wrong: %+v", got)
	}

	updated, err := svc.Update(created.Id, SubscriptionInput{
		Remark:  "cdn",
		MaxIps:  1,
		Enabled: &enabled,
		Inbounds: []InboundInput{{
			InboundId:     10,
			CdnTLS:        true,
			CdnServer:     "203.0.113.20",
			CdnPort:       8443,
			CdnServerName: "edge.example.com",
			CdnXHTTPHost:  "host.example.com",
		}},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	got = updated.Inbounds[0]
	if got.CdnServer != "203.0.113.20" || got.CdnPort != 8443 || got.CdnServerName != "edge.example.com" || got.CdnXHTTPHost != "host.example.com" {
		t.Fatalf("cdn override update not persisted: %+v", got)
	}
}

func TestSubscriptionCDNTLSOverrideRequiresServer(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	_, err := svc.Create(SubscriptionInput{
		Remark:   "bad",
		Inbounds: []InboundInput{{InboundId: 1, CdnTLS: true}},
	})
	if err != ErrCDNServerRequired {
		t.Fatalf("err = %v, want ErrCDNServerRequired", err)
	}

	_, err = svc.Create(SubscriptionInput{
		Remark:       "bad-stats",
		TrafficStats: true,
		Inbounds:     []InboundInput{{InboundId: 10, CdnTLS: true}},
	})
	if err != ErrCDNServerRequired {
		t.Fatalf("err = %v, want ErrCDNServerRequired with traffic stats enabled", err)
	}
}

func TestSubscriptionInfersSharedClientEmail(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	created, err := svc.Create(SubscriptionInput{
		Remark:       "shared",
		TrafficStats: true,
		Inbounds: []InboundInput{
			{InboundId: 1},
			{InboundId: 2},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(created.Inbounds) != 2 {
		t.Fatalf("inbounds len = %d, want 2", len(created.Inbounds))
	}
	for _, inbound := range created.Inbounds {
		if inbound.ClientEmail != "alice@x" {
			t.Fatalf("clientEmail = %q, want alice@x", inbound.ClientEmail)
		}
	}
}

func TestSubscriptionRejectsInboundsWithoutCommonClient(t *testing.T) {
	setupTestDB(t)
	seedSubscriptionTestInbounds(t, map[int][]xmodel.Client{
		20: {testClient("uuid-20", "alice@x")},
		21: {testClient("uuid-21", "bob@x")},
	})
	svc := NewSubscriptionService()

	_, err := svc.Create(SubscriptionInput{
		Remark:       "split",
		TrafficStats: true,
		Inbounds: []InboundInput{
			{InboundId: 20},
			{InboundId: 21},
		},
	})
	if err != ErrCommonClientRequired {
		t.Fatalf("err = %v, want ErrCommonClientRequired", err)
	}
}

func TestSubscriptionRejectsAmbiguousCommonClient(t *testing.T) {
	setupTestDB(t)
	clients := []xmodel.Client{
		testClient("uuid-a", "alice@x"),
		testClient("uuid-b", "bob@x"),
	}
	seedSubscriptionTestInbounds(t, map[int][]xmodel.Client{
		22: clients,
		23: clients,
	})
	svc := NewSubscriptionService()

	_, err := svc.Create(SubscriptionInput{
		Remark:       "ambiguous",
		TrafficStats: true,
		Inbounds: []InboundInput{
			{InboundId: 22},
			{InboundId: 23},
		},
	})
	if err != ErrCommonClientAmbiguous {
		t.Fatalf("err = %v, want ErrCommonClientAmbiguous", err)
	}
}

func TestSubscriptionRejectsSelectedClientMissingFromInbound(t *testing.T) {
	setupTestDB(t)
	seedSubscriptionTestInbounds(t, map[int][]xmodel.Client{
		24: {testClient("uuid-24", "alice@x")},
		25: {testClient("uuid-25", "bob@x")},
	})
	svc := NewSubscriptionService()

	_, err := svc.Create(SubscriptionInput{
		Remark:       "bad-client",
		TrafficStats: true,
		Inbounds: []InboundInput{
			{InboundId: 24, ClientEmail: "alice@x"},
			{InboundId: 25, ClientEmail: "alice@x"},
		},
	})
	if err != ErrSelectedClientInvalid {
		t.Fatalf("err = %v, want ErrSelectedClientInvalid", err)
	}
}

func TestSubscriptionAllowsMixedClientsWhenTrafficStatsDisabled(t *testing.T) {
	setupTestDB(t)
	seedSubscriptionTestInbounds(t, map[int][]xmodel.Client{
		26: {testClient("uuid-26", "alice@x")},
		27: {testClient("uuid-27", "bob@x")},
	})
	svc := NewSubscriptionService()

	created, err := svc.Create(SubscriptionInput{
		Remark: "mixed-no-stats",
		Inbounds: []InboundInput{
			{InboundId: 26, ClientEmail: "alice@x"},
			{InboundId: 27, ClientEmail: "bob@x"},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.TrafficStats {
		t.Fatal("trafficStats = true, want false")
	}
	for _, inbound := range created.Inbounds {
		if inbound.ClientEmail != "" {
			t.Fatalf("clientEmail = %q, want empty when traffic stats disabled", inbound.ClientEmail)
		}
	}

	updated, err := svc.Update(created.Id, SubscriptionInput{
		Remark: "mixed-no-stats-update",
		Inbounds: []InboundInput{
			{InboundId: 26, ClientEmail: "alice@x"},
		},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(updated.Inbounds) != 1 {
		t.Fatalf("updated inbounds len = %d, want 1", len(updated.Inbounds))
	}
	if updated.Inbounds[0].ClientEmail != "" {
		t.Fatalf("updated clientEmail = %q, want empty when traffic stats disabled", updated.Inbounds[0].ClientEmail)
	}
}

func TestResetTokenClearsBindingsStatsAndLogs(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()
	enabled := true
	created, err := svc.Create(SubscriptionInput{
		Remark:   "recover",
		MaxIps:   2,
		Enabled:  &enabled,
		Inbounds: []InboundInput{{InboundId: 1}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	oldToken := created.Token
	ipSvc := NewIPBindingService()
	if err := ipSvc.Enforce(created.Id, created.MaxIps, "1.1.1.1"); err != nil {
		t.Fatalf("bind ip: %v", err)
	}
	if err := NewSubscriptionUsageService().RecordCompleted(created.Id, "1.1.1.1", "mihomo"); err != nil {
		t.Fatalf("record stats: %v", err)
	}
	if err := NewAccessLogService().Record(AccessLogInput{
		SubscriptionId: created.Id,
		Endpoint:       AccessEndpointFull,
		Ip:             "1.1.1.1",
		UserAgent:      "mihomo",
		StatusCode:     200,
		Result:         AccessResultSuccess,
	}); err != nil {
		t.Fatalf("record log: %v", err)
	}

	reset, err := svc.ResetToken(created.Id)
	if err != nil {
		t.Fatalf("reset token: %v", err)
	}
	if reset.Token == oldToken {
		t.Fatal("token did not change")
	}
	if reset.BoundIpCount != 0 {
		t.Fatalf("bound ip count after reset = %d, want 0", reset.BoundIpCount)
	}
	if found, err := svc.FindByToken(oldToken); err != nil || found != nil {
		t.Fatalf("old token lookup = %+v, %v; want nil, nil", found, err)
	}
	if found, err := svc.FindByToken(reset.Token); err != nil || found == nil {
		t.Fatalf("new token lookup = %+v, %v; want subscription, nil", found, err)
	}
	var bindingCount int64
	if err := database.GetDB().Model(&model.IpBinding{}).
		Where("subscription_id = ?", created.Id).
		Count(&bindingCount).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("bindings after reset = %d, want 0", bindingCount)
	}
	var statsCount int64
	if err := database.GetDB().Model(&model.SubscriptionStats{}).
		Where("subscription_id = ?", created.Id).
		Count(&statsCount).Error; err != nil {
		t.Fatalf("count stats: %v", err)
	}
	if statsCount != 0 {
		t.Fatalf("stats rows after reset = %d, want 0", statsCount)
	}
	var logCount int64
	if err := database.GetDB().Model(&model.AccessLog{}).
		Where("subscription_id = ?", created.Id).
		Count(&logCount).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("logs after reset = %d, want 0", logCount)
	}
}

func TestGetMissingReturnsNotFound(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	if _, err := svc.Get(9999); err != ErrSubscriptionNotFound {
		t.Fatalf("expected ErrSubscriptionNotFound, got %v", err)
	}
}

func TestSubscriptionListMapsBoundIPCounts(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()
	enabled := true

	first, err := svc.Create(SubscriptionInput{
		Remark:   "first",
		MaxIps:   10,
		Enabled:  &enabled,
		Inbounds: []InboundInput{{InboundId: 1}},
	})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := svc.Create(SubscriptionInput{
		Remark:   "second",
		MaxIps:   10,
		Enabled:  &enabled,
		Inbounds: []InboundInput{{InboundId: 1}},
	})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	third, err := svc.Create(SubscriptionInput{
		Remark:   "third",
		MaxIps:   10,
		Enabled:  &enabled,
		Inbounds: []InboundInput{{InboundId: 1}},
	})
	if err != nil {
		t.Fatalf("create third: %v", err)
	}

	ipSvc := NewIPBindingService()
	for _, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		if err := ipSvc.Enforce(first.Id, first.MaxIps, ip); err != nil {
			t.Fatalf("bind first %s: %v", ip, err)
		}
	}
	if err := ipSvc.Enforce(second.Id, second.MaxIps, "3.3.3.3"); err != nil {
		t.Fatalf("bind second: %v", err)
	}

	subs, err := svc.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := make(map[int]int64, len(subs))
	for _, sub := range subs {
		got[sub.Id] = sub.BoundIpCount
	}
	want := map[int]int64{
		first.Id:  2,
		second.Id: 1,
		third.Id:  0,
	}
	for id, count := range want {
		if got[id] != count {
			t.Fatalf("bound ip count for sub %d = %d, want %d (all=%v)", id, got[id], count, got)
		}
	}
}

func TestTokensAreUnique(t *testing.T) {
	setupTestDB(t)
	svc := NewSubscriptionService()

	seen := make(map[string]struct{}, 20)
	for i := 0; i < 20; i++ {
		sub, err := svc.Create(SubscriptionInput{
			Remark:   "n",
			Inbounds: []InboundInput{{InboundId: 1}},
		})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		if _, dup := seen[sub.Token]; dup {
			t.Fatalf("duplicate token at iteration %d: %s", i, sub.Token)
		}
		seen[sub.Token] = struct{}{}
	}
}
