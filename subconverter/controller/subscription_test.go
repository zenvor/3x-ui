package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
	"github.com/mhsanaei/3x-ui/v3/subconverter/service"
)

func setupSubscriptionControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := database.Reset(); err != nil {
		t.Fatalf("reset db: %v", err)
	}
	if err := database.InitDB(); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.Reset() })

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewSubscriptionController(engine.Group("/panel/api/subconverter"))
	return engine
}

func TestUpdateSettingsBindsFormPayload(t *testing.T) {
	engine := setupSubscriptionControllerTest(t)
	body := strings.NewReader("uaFilterEnabled=true&uaKeywords=Clash&uaKeywords=Shadowrocket&uaRejectStatus=404")
	req := httptest.NewRequest(http.MethodPost, "/panel/api/subconverter/settings", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("settings status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	var payload struct {
		Success bool `json:"success"`
		Obj     struct {
			UAFilterEnabled bool     `json:"uaFilterEnabled"`
			UAKeywords      []string `json:"uaKeywords"`
			UARejectStatus  int      `json:"uaRejectStatus"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, resp.Body.String())
	}
	if !payload.Success {
		t.Fatalf("settings response success=false; body=%s", resp.Body.String())
	}
	if !payload.Obj.UAFilterEnabled {
		t.Fatal("uaFilterEnabled = false, want true")
	}
	if payload.Obj.UARejectStatus != http.StatusNotFound {
		t.Fatalf("uaRejectStatus = %d, want 404", payload.Obj.UARejectStatus)
	}
	want := []string{"clash", "shadowrocket"}
	if len(payload.Obj.UAKeywords) != len(want) {
		t.Fatalf("uaKeywords = %#v, want %#v", payload.Obj.UAKeywords, want)
	}
	for i := range want {
		if payload.Obj.UAKeywords[i] != want[i] {
			t.Fatalf("uaKeywords = %#v, want %#v", payload.Obj.UAKeywords, want)
		}
	}
}

func TestSubscriptionManagementEndpoints(t *testing.T) {
	engine := setupSubscriptionControllerTest(t)
	sub := &model.Subscription{
		Token:   "manage-token",
		Remark:  "manage",
		MaxIps:  2,
		Enabled: true,
	}
	if err := database.GetDB().Create(sub).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	ip := &model.IpBinding{SubscriptionId: sub.Id, Ip: "1.1.1.1"}
	if err := database.GetDB().Create(ip).Error; err != nil {
		t.Fatalf("create ip binding: %v", err)
	}
	if err := service.NewAccessLogService().Record(service.AccessLogInput{
		SubscriptionId: sub.Id,
		Endpoint:       service.AccessEndpointFull,
		Ip:             "1.1.1.1",
		UserAgent:      "mihomo",
		StatusCode:     http.StatusOK,
		Result:         service.AccessResultSuccess,
	}); err != nil {
		t.Fatalf("record access log: %v", err)
	}
	if err := service.NewSubscriptionUsageService().RecordCompleted(sub.Id, "1.1.1.1", "mihomo"); err != nil {
		t.Fatalf("record stats: %v", err)
	}

	detailResp := performAPIRequest(engine, http.MethodGet, fmt.Sprintf("/panel/api/subconverter/get/%d", sub.Id), nil)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", detailResp.Code)
	}
	var detailPayload struct {
		Success bool `json:"success"`
		Obj     struct {
			model.Subscription
			BoundIps   []model.IpBinding `json:"boundIps"`
			AccessLogs []model.AccessLog `json:"accessLogs"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(detailResp.Body.Bytes(), &detailPayload); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if !detailPayload.Success || detailPayload.Obj.Id != sub.Id || detailPayload.Obj.Token != "manage-token" {
		t.Fatalf("unexpected detail subscription: %s", detailResp.Body.String())
	}
	if len(detailPayload.Obj.BoundIps) != 1 || detailPayload.Obj.BoundIps[0].Ip != "1.1.1.1" {
		t.Fatalf("unexpected detail bound ips: %s", detailResp.Body.String())
	}
	if len(detailPayload.Obj.AccessLogs) != 1 || detailPayload.Obj.AccessLogs[0].Result != service.AccessResultSuccess {
		t.Fatalf("unexpected detail access logs: %s", detailResp.Body.String())
	}
	if detailPayload.Obj.Stats == nil || detailPayload.Obj.Stats.CompletedCount != 1 {
		t.Fatalf("unexpected detail stats: %s", detailResp.Body.String())
	}

	ipsResp := performAPIRequest(engine, http.MethodGet, fmt.Sprintf("/panel/api/subconverter/ips/%d", sub.Id), nil)
	if ipsResp.Code != http.StatusOK {
		t.Fatalf("ips status = %d, want 200", ipsResp.Code)
	}
	var ipsPayload struct {
		Success bool              `json:"success"`
		Obj     []model.IpBinding `json:"obj"`
	}
	if err := json.Unmarshal(ipsResp.Body.Bytes(), &ipsPayload); err != nil {
		t.Fatalf("decode ips: %v", err)
	}
	if !ipsPayload.Success || len(ipsPayload.Obj) != 1 || ipsPayload.Obj[0].Ip != "1.1.1.1" {
		t.Fatalf("unexpected ips payload: %s", ipsResp.Body.String())
	}

	logsResp := performAPIRequest(engine, http.MethodGet, fmt.Sprintf("/panel/api/subconverter/logs/%d", sub.Id), nil)
	if logsResp.Code != http.StatusOK {
		t.Fatalf("logs status = %d, want 200", logsResp.Code)
	}
	var logsPayload struct {
		Success bool              `json:"success"`
		Obj     []model.AccessLog `json:"obj"`
	}
	if err := json.Unmarshal(logsResp.Body.Bytes(), &logsPayload); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	if !logsPayload.Success || len(logsPayload.Obj) != 1 || logsPayload.Obj[0].Result != service.AccessResultSuccess {
		t.Fatalf("unexpected logs payload: %s", logsResp.Body.String())
	}

	allLogsResp := performAPIRequest(engine, http.MethodGet, "/panel/api/subconverter/logs?limit=10", nil)
	if allLogsResp.Code != http.StatusOK {
		t.Fatalf("all logs status = %d, want 200", allLogsResp.Code)
	}
	var allLogsPayload struct {
		Success bool `json:"success"`
		Obj     []struct {
			model.AccessLog
			SubscriptionRemark string `json:"subscriptionRemark"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(allLogsResp.Body.Bytes(), &allLogsPayload); err != nil {
		t.Fatalf("decode all logs: %v", err)
	}
	if !allLogsPayload.Success || len(allLogsPayload.Obj) != 1 || allLogsPayload.Obj[0].SubscriptionRemark != "manage" {
		t.Fatalf("unexpected all logs payload: %s", allLogsResp.Body.String())
	}

	delResp := performAPIRequest(engine, http.MethodPost, fmt.Sprintf("/panel/api/subconverter/ips/%d/del/%d", sub.Id, ip.Id), nil)
	if delResp.Code != http.StatusOK {
		t.Fatalf("delete ip status = %d, want 200", delResp.Code)
	}
	var count int64
	if err := database.GetDB().Model(&model.IpBinding{}).Where("subscription_id = ?", sub.Id).Count(&count).Error; err != nil {
		t.Fatalf("count bindings after delete: %v", err)
	}
	if count != 0 {
		t.Fatalf("bindings after delete = %d, want 0", count)
	}
	if err := database.GetDB().Create(&model.IpBinding{SubscriptionId: sub.Id, Ip: "2.2.2.2"}).Error; err != nil {
		t.Fatalf("recreate binding: %v", err)
	}

	resetResp := performAPIRequest(engine, http.MethodPost, fmt.Sprintf("/panel/api/subconverter/reset-token/%d", sub.Id), nil)
	if resetResp.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200", resetResp.Code)
	}
	var resetPayload struct {
		Success bool               `json:"success"`
		Obj     model.Subscription `json:"obj"`
	}
	if err := json.Unmarshal(resetResp.Body.Bytes(), &resetPayload); err != nil {
		t.Fatalf("decode reset: %v", err)
	}
	if !resetPayload.Success || resetPayload.Obj.Token == "manage-token" {
		t.Fatalf("unexpected reset payload: %s", resetResp.Body.String())
	}
	if err := database.GetDB().Model(&model.IpBinding{}).Where("subscription_id = ?", sub.Id).Count(&count).Error; err != nil {
		t.Fatalf("count bindings after reset: %v", err)
	}
	if count != 0 {
		t.Fatalf("bindings after reset = %d, want 0", count)
	}
	if err := database.GetDB().Model(&model.AccessLog{}).Where("subscription_id = ?", sub.Id).Count(&count).Error; err != nil {
		t.Fatalf("count logs after reset: %v", err)
	}
	if count != 0 {
		t.Fatalf("logs after reset = %d, want 0", count)
	}
	if err := database.GetDB().Model(&model.SubscriptionStats{}).Where("subscription_id = ?", sub.Id).Count(&count).Error; err != nil {
		t.Fatalf("count stats after reset: %v", err)
	}
	if count != 0 {
		t.Fatalf("stats after reset = %d, want 0", count)
	}
	resetDetailResp := performAPIRequest(engine, http.MethodGet, fmt.Sprintf("/panel/api/subconverter/get/%d", sub.Id), nil)
	if resetDetailResp.Code != http.StatusOK {
		t.Fatalf("detail after reset status = %d, want 200", resetDetailResp.Code)
	}
	var resetDetailPayload struct {
		Success bool `json:"success"`
		Obj     struct {
			model.Subscription
			BoundIps   []model.IpBinding `json:"boundIps"`
			AccessLogs []model.AccessLog `json:"accessLogs"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(resetDetailResp.Body.Bytes(), &resetDetailPayload); err != nil {
		t.Fatalf("decode detail after reset: %v", err)
	}
	if !resetDetailPayload.Success || resetDetailPayload.Obj.Token != resetPayload.Obj.Token {
		t.Fatalf("unexpected detail after reset subscription: %s", resetDetailResp.Body.String())
	}
	if len(resetDetailPayload.Obj.BoundIps) != 0 || len(resetDetailPayload.Obj.AccessLogs) != 0 {
		t.Fatalf("detail after reset retained rows: %s", resetDetailResp.Body.String())
	}
	if resetDetailPayload.Obj.Stats != nil && resetDetailPayload.Obj.Stats.CompletedCount != 0 {
		t.Fatalf("detail after reset retained stats: %s", resetDetailResp.Body.String())
	}

	if err := database.GetDB().Create(&model.IpBinding{SubscriptionId: sub.Id, Ip: "3.3.3.3"}).Error; err != nil {
		t.Fatalf("recreate binding for clear: %v", err)
	}
	clearResp := performAPIRequest(engine, http.MethodPost, fmt.Sprintf("/panel/api/subconverter/ips/clear/%d", sub.Id), nil)
	if clearResp.Code != http.StatusOK {
		t.Fatalf("clear status = %d, want 200", clearResp.Code)
	}
	if err := database.GetDB().Model(&model.IpBinding{}).Where("subscription_id = ?", sub.Id).Count(&count).Error; err != nil {
		t.Fatalf("count bindings after clear: %v", err)
	}
	if count != 0 {
		t.Fatalf("bindings after clear = %d, want 0", count)
	}
}

func TestDeleteIPEndpointRequiresSubscriptionScope(t *testing.T) {
	engine := setupSubscriptionControllerTest(t)
	first := &model.Subscription{Token: "scope-token-a", Remark: "first", MaxIps: 2, Enabled: true}
	second := &model.Subscription{Token: "scope-token-b", Remark: "second", MaxIps: 2, Enabled: true}
	if err := database.GetDB().Create(first).Error; err != nil {
		t.Fatalf("create first subscription: %v", err)
	}
	if err := database.GetDB().Create(second).Error; err != nil {
		t.Fatalf("create second subscription: %v", err)
	}
	binding := &model.IpBinding{SubscriptionId: first.Id, Ip: "1.1.1.1"}
	if err := database.GetDB().Create(binding).Error; err != nil {
		t.Fatalf("create ip binding: %v", err)
	}

	resp := performAPIRequest(engine, http.MethodPost, fmt.Sprintf("/panel/api/subconverter/ips/%d/del/%d", second.Id, binding.Id), nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("delete ip status = %d, want 200", resp.Code)
	}
	var payload struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if payload.Success {
		t.Fatalf("cross-subscription delete succeeded unexpectedly: %s", resp.Body.String())
	}
	var count int64
	if err := database.GetDB().Model(&model.IpBinding{}).Where("id = ? AND subscription_id = ?", binding.Id, first.Id).Count(&count).Error; err != nil {
		t.Fatalf("count original binding: %v", err)
	}
	if count != 1 {
		t.Fatalf("original binding count = %d, want 1", count)
	}
}

func performAPIRequest(engine http.Handler, method string, path string, body *strings.Reader) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		reader = body
	}
	req := httptest.NewRequest(method, path, reader)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	return resp
}
