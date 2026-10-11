package controller

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
	"github.com/mhsanaei/3x-ui/v3/subconverter/service"
)

func TestUnavailableTemplateDoesNotBreakProviderUAPolicy(t *testing.T) {
	engine := setupPublicControllerTest(t)
	sub := createPublicTestSubscription(t, 1)
	if _, err := service.NewSettingsService().Get(); err != nil {
		t.Fatal(err)
	}
	const missing = "remote-11111111-1111-4111-8111-111111111111"
	if err := database.GetDB().Model(&model.Settings{}).Where("id = 1").Update("template_variant", missing).Error; err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []struct {
		suffix string
		status int
	}{
		{suffix: "/nodes", status: http.StatusOK},
		{suffix: "", status: http.StatusServiceUnavailable},
	} {
		allowed := performFeedRequestWithUA(engine, "/feed/"+sub.Token+endpoint.suffix, "1.1.1.1", "mihomo")
		if allowed.Code != endpoint.status {
			t.Fatalf("endpoint %q: status %d, body %s", endpoint.suffix, allowed.Code, allowed.Body.String())
		}
		denied := performFeedRequestWithUA(engine, "/feed/"+sub.Token+endpoint.suffix, "1.1.1.1", "curl")
		if denied.Code != http.StatusForbidden {
			t.Fatalf("missing template bypassed UA policy: %d", denied.Code)
		}
	}
}

func TestCustomTemplateCatalogAPIAndPublicFeed(t *testing.T) {
	engine := setupPublicControllerTest(t)
	NewSubscriptionController(engine.Group("/panel/api/subconverter"))
	sub := createPublicTestSubscription(t, 1)
	const id = "remote-12345678-1234-1234-1234-123456789abc"
	const sourceURL = "https://templates.example/config.yaml"
	if err := database.GetDB().Create(&model.Template{ID: id, Name: "Custom", Source: service.TemplateSourceRemote, URL: sourceURL}).Error; err != nil {
		t.Fatal(err)
	}
	body := []byte(publicTestTemplate + "profile-name: custom\n")
	cachePath := filepath.Join(filepath.Dir(database.GetDBPath()), fmt.Sprintf("mihomo-template-%s-%x.yaml", id, sha256.Sum256([]byte(sourceURL))))
	if err := os.WriteFile(cachePath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(map[string]any{"url": sourceURL, "cacheSHA256": fmt.Sprintf("%x", sha256.Sum256(body)), "checkedAt": int64(1234567890000), "checkResult": "updated"})
	if err := os.WriteFile(cachePath+".check.json", metadata, 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := service.NewSettingsService().Get()
	if err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(&model.Settings{}).Where("id = 1").Update("template_variant", id).Error; err != nil {
		t.Fatal(err)
	}
	// Seed an already-active custom source, so this test exercises handler binding
	// and public rendering without depending on external HTTPS downloads.
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/panel/api/subconverter"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		engine.ServeHTTP(resp, req)
		return resp
	}
	assertSuccess := func(resp *httptest.ResponseRecorder, want bool) {
		t.Helper()
		var msg struct {
			Success bool
			Msg     string
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &msg); err != nil || msg.Success != want {
			t.Fatalf("response %s (%v)", resp.Body.String(), err)
		}
		if want && msg.Msg != "" {
			t.Fatalf("success contained failure message: %s", resp.Body.String())
		}
	}
	list := request(http.MethodGet, "/templates", "")
	assertSuccess(list, true)
	var catalog struct{ Obj []service.TemplateRecord }
	if err := json.Unmarshal(list.Body.Bytes(), &catalog); err != nil || len(catalog.Obj) != 3 || catalog.Obj[2].ID != id || catalog.Obj[2].CheckedAt != 1234567890000 {
		t.Fatalf("list %s %v", list.Body.String(), err)
	}
	save := request(http.MethodPost, "/settings", fmt.Sprintf(`{"uaFilterEnabled":%t,"uaKeywords":["mihomo"],"uaRejectStatus":403,"activeTemplateId":%q}`, settings.UAFilterEnabled, id))
	assertSuccess(save, true)
	var saved struct{ Obj service.SettingsDTO }
	if err := json.Unmarshal(save.Body.Bytes(), &saved); err != nil || saved.Obj.ActiveTemplateID != id {
		t.Fatalf("settings %s %v", save.Body.String(), err)
	}
	renamed := request(http.MethodPost, "/templates/"+id, fmt.Sprintf(`{"name":"Renamed","url":%q,"source":"remote"}`, sourceURL))
	assertSuccess(renamed, true)
	activeStatus := request(http.MethodGet, "/template", "")
	var status struct{ Obj service.TemplateStatus }
	if err := json.Unmarshal(activeStatus.Body.Bytes(), &status); err != nil || status.Obj.Name != "Renamed" || status.Obj.ID != id || status.Obj.CheckedAt != 1234567890000 {
		t.Fatalf("status %s %v", activeStatus.Body.String(), err)
	}
	feed := performFeedRequest(engine, "/feed/"+sub.Token, "1.1.1.1")
	if feed.Code != http.StatusOK || !strings.Contains(feed.Body.String(), "profile-name: custom") || strings.Contains(feed.Body.String(), "__TOKEN__") {
		t.Fatalf("feed %d %s", feed.Code, feed.Body.String())
	}
	assertSuccess(request(http.MethodPost, "/templates/"+id+"/delete", ""), false)
	assertSuccess(request(http.MethodPost, "/templates/general/delete", ""), false)
	assertSuccess(request(http.MethodPost, "/templates", `{"name":"Upload","url":"https://templates.example/file.yaml","source":"upload"}`), false)
	assertSuccess(request(http.MethodPost, "/templates", `{"name":"Private","url":"https://127.0.0.1/template.yaml"}`), false)
	if err := database.GetDB().Model(&model.Settings{}).Where("id = 1").Update("template_variant", "general").Error; err != nil {
		t.Fatal(err)
	}
	assertSuccess(request(http.MethodPost, "/templates/"+id+"/delete", ""), true)
}
