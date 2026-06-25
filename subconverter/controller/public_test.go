package controller

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	logging "github.com/op/go-logging"

	xdatabase "github.com/mhsanaei/3x-ui/v3/internal/database"
	xmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

func setupPublicControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	t.Setenv("XUI_LOG_FOLDER", tmpDir)
	logger.InitLogger(logging.CRITICAL)
	if err := database.Reset(); err != nil {
		t.Fatalf("reset db: %v", err)
	}
	if err := database.InitDB(); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Reset()
		logger.CloseLogger()
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewPublicController(engine)
	return engine
}

func TestFeedSuccessRecordsSubscriptionCount(t *testing.T) {
	engine := setupPublicControllerTest(t)
	sub := createPublicTestSubscription(t, 1)

	resp := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
	if resp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	assertCompletedCount(t, sub.Id, 1)
}

func TestFeedIncludesWebRTCLeakProtectionRulesByDefault(t *testing.T) {
	engine := setupPublicControllerTest(t)
	sub := createPublicTestSubscription(t, 1)

	resp := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
	if resp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	for _, rule := range []string{
		"AND,((NETWORK,udp),(DST-PORT,3478-3481)),REJECT",
		"AND,((NETWORK,UDP),(DST-PORT,443)),REJECT",
		"NETWORK,udp,REJECT",
	} {
		if !strings.Contains(resp.Body.String(), rule) {
			t.Fatalf("expected leak protection rule %q in feed body:\n%s", rule, resp.Body.String())
		}
	}
}

func TestFeedRejectsDisallowedUserAgent(t *testing.T) {
	engine := setupPublicControllerTest(t)
	sub := createPublicTestSubscription(t, 1)

	resp := performFeedRequestWithUA(engine, "/feed/"+sub.Token, "1.1.1.1", "Mozilla/5.0")

	if resp.Code != http.StatusForbidden {
		t.Fatalf("feed status = %d, want 403; body=%s", resp.Code, resp.Body.String())
	}
	assertCompletedCount(t, sub.Id, 0)
}

func TestDisallowedUserAgentDoesNotRevealKnownToken(t *testing.T) {
	cases := []struct {
		name     string
		suffix   string
		endpoint string
	}{
		{name: "full", suffix: "", endpoint: "full"},
		{name: "nodes", suffix: "/nodes", endpoint: "nodes"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			engine := setupPublicControllerTest(t)
			sub := createPublicTestSubscription(t, 1)

			known := performFeedRequestWithUA(engine, "/feed/"+sub.Token+tt.suffix, "1.1.1.1", "Mozilla/5.0")
			unknown := performFeedRequestWithUA(engine, "/feed/missing-token"+tt.suffix, "1.1.1.1", "Mozilla/5.0")

			if known.Code != unknown.Code {
				t.Fatalf("known token status = %d, unknown token status = %d; want equal", known.Code, unknown.Code)
			}
			assertAccessLogCount(t, sub.Id, 1)
			assertLatestAccessLog(t, sub.Id, tt.endpoint, "ua_rejected", known.Code, "1.1.1.1")

			var total int64
			if err := database.GetDB().Model(&model.AccessLog{}).Count(&total).Error; err != nil {
				t.Fatalf("count logs: %v", err)
			}
			if total != 1 {
				t.Fatalf("total access logs = %d, want 1", total)
			}
		})
	}
}

func TestProviderUserAgentPolicy(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		want int
	}{
		{name: "mihomo", ua: "mihomo/1.19.0", want: http.StatusOK},
		{name: "clash meta android", ua: "ClashMetaForAndroid/2.11.23.Meta", want: http.StatusOK},
		{name: "shadowrocket", ua: "Shadowrocket/1996 CFNetwork", want: http.StatusOK},
		{name: "empty", ua: "", want: http.StatusForbidden},
		{name: "curl", ua: "curl/8.7.1", want: http.StatusForbidden},
		{name: "browser", ua: "Mozilla/5.0", want: http.StatusForbidden},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			engine := setupPublicControllerTest(t)
			sub := createPublicTestSubscription(t, 1)

			resp := performFeedRequestWithUA(engine, "/feed/"+sub.Token+"/nodes", "1.1.1.1", tt.ua)
			if resp.Code != tt.want {
				t.Fatalf("provider status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body.String())
			}
			assertCompletedCount(t, sub.Id, 0)
		})
	}
}

func TestProviderDoesNotRecordSubscriptionCount(t *testing.T) {
	engine := setupPublicControllerTest(t)
	sub := createPublicTestSubscription(t, 1)

	resp := performFeedRequest(engine, "/feed/"+sub.Token+"/nodes", "1.1.1.1")
	if resp.Code != http.StatusOK {
		t.Fatalf("provider status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	assertCompletedCount(t, sub.Id, 0)
}

func TestFeedSetsSubscriptionUserinfoHeader(t *testing.T) {
	engine := setupPublicControllerTest(t)
	setupPublicControllerMainDB(t)
	sub := createPublicTestSubscriptionWithInbounds(t, "alice@x", 1, 2)
	enablePublicTestTrafficStats(t, sub)
	seedPublicControllerTraffic(t, "alice@x", 123, 456, 7890, 1700000000000)

	resp := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
	if resp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	want := "upload=123; download=456; total=7890; expire=1700000000"
	if got := resp.Header().Get("Subscription-Userinfo"); got != want {
		t.Fatalf("Subscription-Userinfo = %q, want %q", got, want)
	}
}

func TestProviderSetsSubscriptionUserinfoHeader(t *testing.T) {
	engine := setupPublicControllerTest(t)
	setupPublicControllerMainDB(t)
	sub := createPublicTestSubscriptionWithInbounds(t, "alice@x", 1, 2)
	enablePublicTestTrafficStats(t, sub)
	seedPublicControllerTraffic(t, "alice@x", 11, 22, 33, 1700000000000)

	resp := performFeedRequest(engine, "/feed/"+sub.Token+"/nodes", "1.1.1.1")
	if resp.Code != http.StatusOK {
		t.Fatalf("provider status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	want := "upload=11; download=22; total=33; expire=1700000000"
	if got := resp.Header().Get("Subscription-Userinfo"); got != want {
		t.Fatalf("Subscription-Userinfo = %q, want %q", got, want)
	}
}

func TestFeedOmitsSubscriptionUserinfoForAmbiguousLegacyClient(t *testing.T) {
	engine := setupPublicControllerTest(t)
	setupPublicControllerMainDB(t)
	sub := createPublicTestSubscription(t, 1)
	enablePublicTestTrafficStats(t, sub)
	createPublicTestSubscriptionInbound(t, sub.Id, 1, "")

	resp := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
	if resp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Subscription-Userinfo"); got != "" {
		t.Fatalf("Subscription-Userinfo = %q, want empty", got)
	}
}

func TestFeedOmitsSubscriptionUserinfoWhenTrafficStatsDisabled(t *testing.T) {
	engine := setupPublicControllerTest(t)
	setupPublicControllerMainDB(t)
	sub := createPublicTestSubscriptionWithInbounds(t, "alice@x", 1, 2)
	seedPublicControllerTraffic(t, "alice@x", 123, 456, 7890, 1700000000000)

	resp := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
	if resp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Subscription-Userinfo"); got != "" {
		t.Fatalf("Subscription-Userinfo = %q, want empty", got)
	}
}

func TestFeedOmitsSubscriptionUserinfoWhenStoredClientIsNotExportable(t *testing.T) {
	engine := setupPublicControllerTest(t)
	setupPublicControllerMainDB(t)
	sub := createPublicTestSubscriptionWithInbounds(t, "missing@x", 1, 2)
	enablePublicTestTrafficStats(t, sub)
	seedPublicControllerTraffic(t, "missing@x", 123, 456, 7890, 1700000000000)

	resp := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
	if resp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Subscription-Userinfo"); got != "" {
		t.Fatalf("Subscription-Userinfo = %q, want empty", got)
	}
}

func TestFeedForbiddenDoesNotRecordSubscriptionCount(t *testing.T) {
	engine := setupPublicControllerTest(t)
	sub := createPublicTestSubscription(t, 1)

	first := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
	if first.Code != http.StatusOK {
		t.Fatalf("first feed status = %d, want 200; body=%s", first.Code, first.Body.String())
	}
	second := performFeedRequest(engine, "/feed/"+sub.Token, "2.2.2.2")
	if second.Code != http.StatusForbidden {
		t.Fatalf("second feed status = %d, want 403; body=%s", second.Code, second.Body.String())
	}
	assertCompletedCount(t, sub.Id, 1)
}

func TestFeedAccessLogsKnownSubscriptionOutcomes(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		engine := setupPublicControllerTest(t)
		sub := createPublicTestSubscription(t, 1)

		resp := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
		if resp.Code != http.StatusOK {
			t.Fatalf("feed status = %d, want 200", resp.Code)
		}
		assertLatestAccessLog(t, sub.Id, "full", "success", http.StatusOK, "1.1.1.1")
		assertAccessLogCount(t, sub.Id, 1)
	})

	t.Run("ua rejected", func(t *testing.T) {
		engine := setupPublicControllerTest(t)
		sub := createPublicTestSubscription(t, 1)

		resp := performFeedRequestWithUA(engine, "/feed/"+sub.Token, "1.1.1.1", "Mozilla/5.0")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("feed status = %d, want 403", resp.Code)
		}
		assertLatestAccessLog(t, sub.Id, "full", "ua_rejected", http.StatusForbidden, "1.1.1.1")
		assertAccessLogCount(t, sub.Id, 1)
	})

	t.Run("ip limit exceeded", func(t *testing.T) {
		engine := setupPublicControllerTest(t)
		sub := createPublicTestSubscription(t, 1)

		first := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
		if first.Code != http.StatusOK {
			t.Fatalf("first feed status = %d, want 200", first.Code)
		}
		second := performFeedRequest(engine, "/feed/"+sub.Token, "2.2.2.2")
		if second.Code != http.StatusForbidden {
			t.Fatalf("second feed status = %d, want 403", second.Code)
		}
		assertLatestAccessLog(t, sub.Id, "full", "ip_limit_exceeded", http.StatusForbidden, "2.2.2.2")
		assertAccessLogCount(t, sub.Id, 2)
	})

	t.Run("disabled", func(t *testing.T) {
		engine := setupPublicControllerTest(t)
		sub := createPublicTestSubscription(t, 1)
		if err := database.GetDB().Model(sub).Update("enabled", false).Error; err != nil {
			t.Fatalf("disable subscription: %v", err)
		}

		resp := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("feed status = %d, want 404", resp.Code)
		}
		assertLatestAccessLog(t, sub.Id, "full", "subscription_disabled", http.StatusNotFound, "1.1.1.1")
		assertAccessLogCount(t, sub.Id, 1)
	})

	t.Run("nodes endpoint", func(t *testing.T) {
		engine := setupPublicControllerTest(t)
		sub := createPublicTestSubscription(t, 1)

		resp := performFeedRequest(engine, "/feed/"+sub.Token+"/nodes", "1.1.1.1")
		if resp.Code != http.StatusOK {
			t.Fatalf("provider status = %d, want 200", resp.Code)
		}
		assertLatestAccessLog(t, sub.Id, "nodes", "success", http.StatusOK, "1.1.1.1")
		assertAccessLogCount(t, sub.Id, 1)
	})
}

func TestUnknownTokenDoesNotRecordAccessLog(t *testing.T) {
	engine := setupPublicControllerTest(t)

	resp := performFeedRequest(engine, "/feed/ffffffffffffffffffffffffffffffff", "1.1.1.1")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("feed status = %d, want 404", resp.Code)
	}
	var count int64
	if err := database.GetDB().Model(&model.AccessLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if count != 0 {
		t.Fatalf("access logs = %d, want 0", count)
	}
}

func TestMalformedTokenDoesNotRecordAccessLog(t *testing.T) {
	engine := setupPublicControllerTest(t)

	resp := performFeedRequest(engine, "/feed/not-a-token", "1.1.1.1")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("feed status = %d, want 404", resp.Code)
	}
	var count int64
	if err := database.GetDB().Model(&model.AccessLog{}).Count(&count).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if count != 0 {
		t.Fatalf("access logs = %d, want 0", count)
	}
}

func TestFeedIgnoresForwardedHeadersFromUntrustedRemote(t *testing.T) {
	engine := setupPublicControllerTest(t)
	sub := createPublicTestSubscription(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/feed/"+sub.Token, nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("X-Real-IP", "198.51.100.9")
	req.Header.Set("X-Forwarded-For", "198.51.100.8")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "spoof.example")
	req.Header.Set("User-Agent", "mihomo-test")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	assertLatestAccessLog(t, sub.Id, "full", "success", http.StatusOK, "203.0.113.10")
	if strings.Contains(resp.Body.String(), "https://spoof.example/feed/"+sub.Token+"/nodes") {
		t.Fatalf("feed body trusted spoofed forwarded host:\n%s", resp.Body.String())
	}
}

func createPublicTestSubscription(t *testing.T, maxIps int) *model.Subscription {
	t.Helper()
	sub := &model.Subscription{
		Token:   "0123456789abcdef0123456789abcdef",
		Remark:  "mobile",
		MaxIps:  maxIps,
		Enabled: true,
	}
	if err := database.GetDB().Create(sub).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	return sub
}

func createPublicTestSubscriptionWithInbounds(t *testing.T, clientEmail string, inboundIds ...int) *model.Subscription {
	t.Helper()
	sub := createPublicTestSubscription(t, 1)
	for _, inboundID := range inboundIds {
		createPublicTestSubscriptionInbound(t, sub.Id, inboundID, clientEmail)
	}
	return sub
}

func enablePublicTestTrafficStats(t *testing.T, sub *model.Subscription) {
	t.Helper()
	sub.TrafficStats = true
	if err := database.GetDB().Model(sub).Update("traffic_stats", true).Error; err != nil {
		t.Fatalf("enable traffic stats: %v", err)
	}
}

func createPublicTestSubscriptionInbound(t *testing.T, subID, inboundID int, clientEmail string) {
	t.Helper()
	if err := database.GetDB().Create(&model.SubscriptionInbound{
		SubscriptionId: subID,
		InboundId:      inboundID,
		ClientEmail:    clientEmail,
	}).Error; err != nil {
		t.Fatalf("create subscription inbound: %v", err)
	}
}

func setupPublicControllerMainDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := xdatabase.CloseDB(); err != nil {
		t.Fatalf("close main db: %v", err)
	}
	if err := xdatabase.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("init main db: %v", err)
	}
	t.Cleanup(func() { _ = xdatabase.CloseDB() })

	for _, inbound := range []*xmodel.Inbound{
		publicTestInbound(1, []xmodel.Client{
			{ID: "uuid-1", Email: "alice@x", Enable: true},
			{ID: "uuid-1b", Email: "bob@x", Enable: true},
		}),
		publicTestInbound(2, []xmodel.Client{
			{ID: "uuid-2", Email: "alice@x", Enable: true},
		}),
	} {
		if err := xdatabase.GetDB().Create(inbound).Error; err != nil {
			t.Fatalf("seed main inbound %d: %v", inbound.Id, err)
		}
	}
}

func publicTestInbound(id int, clients []xmodel.Client) *xmodel.Inbound {
	settings := `{"clients":[`
	for i, client := range clients {
		if i > 0 {
			settings += `,`
		}
		settings += `{"id":"` + client.ID + `","email":"` + client.Email + `","enable":true}`
	}
	settings += `]}`
	return &xmodel.Inbound{
		Id:       id,
		Remark:   "public-test",
		Tag:      "public-test-inbound-" + strconv.Itoa(id),
		Enable:   true,
		Port:     45000 + id,
		Protocol: xmodel.VLESS,
		StreamSettings: `{
			"network":"tcp",
			"security":"reality",
			"realitySettings":{
				"serverNames":["www.cloudflare.com"],
				"shortIds":["abcd1234"],
				"settings":{"publicKey":"pubkey-xyz","fingerprint":"chrome"}
			}
		}`,
		Settings: settings,
	}
}

func seedPublicControllerTraffic(t *testing.T, email string, up, down, total, expiry int64) {
	t.Helper()
	if err := xdatabase.GetDB().Create(&xray.ClientTraffic{
		InboundId:  1,
		Email:      email,
		Enable:     true,
		Up:         up,
		Down:       down,
		Total:      total,
		ExpiryTime: expiry,
		LastOnline: expiry,
	}).Error; err != nil {
		t.Fatalf("seed client traffic: %v", err)
	}
}

func performFeedRequest(engine http.Handler, path, ip string) *httptest.ResponseRecorder {
	return performFeedRequestWithUA(engine, path, ip, "mihomo-test")
}

func performFeedRequestWithUA(engine http.Handler, path, ip, userAgent string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Real-IP", ip)
	req.Header.Set("User-Agent", userAgent)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	return resp
}

func assertCompletedCount(t *testing.T, subID int, want int64) {
	t.Helper()
	var stats model.SubscriptionStats
	err := database.GetDB().First(&stats, "subscription_id = ?", subID).Error
	if want == 0 {
		if err == nil {
			t.Fatalf("completed count = %d, want no stats row", stats.CompletedCount)
		}
		return
	}
	if err != nil {
		t.Fatalf("load stats: %v", err)
	}
	if stats.CompletedCount != want {
		t.Fatalf("completed count = %d, want %d", stats.CompletedCount, want)
	}
}

func assertLatestAccessLog(t *testing.T, subID int, endpoint, result string, status int, ip string) {
	t.Helper()
	var log model.AccessLog
	if err := database.GetDB().
		Where("subscription_id = ?", subID).
		Order("accessed_at desc, id desc").
		First(&log).Error; err != nil {
		t.Fatalf("load access log: %v", err)
	}
	if log.Endpoint != endpoint || log.Result != result || log.StatusCode != status || log.Ip != ip {
		t.Fatalf("log = %+v, want endpoint=%s result=%s status=%d ip=%s", log, endpoint, result, status, ip)
	}
}

func assertAccessLogCount(t *testing.T, subID int, want int64) {
	t.Helper()
	var count int64
	if err := database.GetDB().Model(&model.AccessLog{}).Where("subscription_id = ?", subID).Count(&count).Error; err != nil {
		t.Fatalf("count access logs: %v", err)
	}
	if count != want {
		t.Fatalf("access logs = %d, want %d", count, want)
	}
}
