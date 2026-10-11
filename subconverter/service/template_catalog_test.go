package service

import (
	"errors"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

func mockRemoteTemplates(t *testing.T, handler templateRoundTripper) {
	t.Helper()
	original := remoteTemplateClient
	remoteTemplateClient = func() *http.Client { return &http.Client{Transport: handler} }
	t.Cleanup(func() { remoteTemplateClient = original })
}

func TestTemplateCatalogIndependentCachesActivationRenameAndRestart(t *testing.T) {
	setupTemplateTestDB(t)
	if _, err := NewSettingsService().Get(); err != nil {
		t.Fatal(err)
	}
	var requests []string
	mockRemoteTemplates(t, func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r.URL.String()+":"+r.Header.Get("If-None-Match"))
		if r.Header.Get("If-None-Match") != "" {
			return templateResponse(304, "", ""), nil
		}
		return templateResponse(200, fetchTestTemplate+"# "+r.URL.Path+"\n", `"same-etag"`), nil
	})
	first, err := SaveTemplate("", TemplateInput{Name: " My template ", URL: "https://templates.example/a.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := SaveTemplate("", TemplateInput{Name: "My template", URL: "https://templates.example/b.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.Name != "My template" || !first.Cached || first.CheckedAt == 0 {
		t.Fatalf("records: %+v %+v", first, second)
	}
	records, err := ListTemplates()
	if err != nil || len(records) != 4 || !records[0].Builtin || !records[1].Builtin {
		t.Fatalf("catalog: %+v %v", records, err)
	}
	// Both custom entries must download even when the upstream ETag is identical.
	if len(requests) != 2 || requests[0] != first.URL+":" || requests[1] != second.URL+":" {
		t.Fatal(requests)
	}
	active := first.ID
	settings, err := NewSettingsService().Update(SettingsInput{UAKeywords: []string{"mihomo"}, ActiveTemplateID: &active})
	if err != nil || settings.ActiveTemplateID != active || settings.TemplateVariant != active {
		t.Fatalf("settings: %+v %v", settings, err)
	}
	content, err := ReadTemplate()
	if err != nil || !strings.Contains(content, "# /a.yaml") {
		t.Fatalf("active content %q %v", content, err)
	}
	yaml, err := RenderMihomoYAML("https://panel.example", "test-token")
	if err != nil || strings.Contains(yaml, "__TOKEN__") || !strings.Contains(yaml, "/feed/test-token/nodes") {
		t.Fatalf("render: %s %v", yaml, err)
	}
	before, err := GetTemplateStatus()
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := SaveTemplate(first.ID, TemplateInput{Name: "Renamed", URL: first.URL})
	if err != nil || renamed.CheckedAt != before.CheckedAt || renamed.UpdatedAt != before.UpdatedAt {
		t.Fatalf("rename changed history %+v %v", renamed, err)
	}
	if len(requests) != 3 {
		t.Fatalf("rename fetched again: %v", requests)
	}
	if err := DeleteTemplate(first.ID); !errors.Is(err, ErrTemplateActive) {
		t.Fatalf("active delete %v", err)
	}
	firstDefinition, _ := resolveTemplate(first.ID)
	secondDefinition, _ := resolveTemplate(second.ID)
	if firstDefinition.cachePath() == secondDefinition.cachePath() {
		t.Fatal("shared custom caches")
	}
	// Refreshing an inactive template cannot overwrite the active source/cache.
	if result, err := RefreshTemplateByID(second.ID); err != nil || result.Changed || result.CheckResult != "unchanged" {
		t.Fatalf("inactive refresh: %+v %v", result, err)
	}
	after, _ := GetTemplateStatus()
	if after.Name != "Renamed" || after.CheckedAt != before.CheckedAt || after.UpdatedAt != before.UpdatedAt {
		t.Fatalf("inactive refresh touched active: %+v", after)
	}
	if err := database.Reset(); err != nil {
		t.Fatal(err)
	}
	templateState.etags = make(map[string]string)
	if err := database.InitDB(); err != nil {
		t.Fatal(err)
	}
	restarted, err := GetTemplateStatus()
	if err != nil || restarted != after {
		t.Fatalf("restart: %+v %v", restarted, err)
	}
	if result, err := RefreshTemplate(); err != nil || result.Changed || result.CheckResult != "unchanged" || result.UpdatedAt != after.UpdatedAt {
		t.Fatalf("restart check: %+v %v", result, err)
	}
	legacy := SettingsInput{UAKeywords: []string{"clash"}}
	if result, err := NewSettingsService().Update(legacy); err != nil || result.ActiveTemplateID != first.ID {
		t.Fatalf("legacy reset custom: %+v %v", result, err)
	}
	if err := DeleteTemplate(second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveTemplate(second.ID); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(&model.Settings{}).Where("id = 1").Update("template_variant", TemplateVariantGeneral).Error; err != nil {
		t.Fatal(err)
	}
	if err := DeleteTemplate(first.ID); err != nil {
		t.Fatal(err)
	}
}

func TestTemplateURLChangeFailureAndDatabaseFailurePreserveActiveSource(t *testing.T) {
	setupTemplateTestDB(t)
	fail := false
	mockRemoteTemplates(t, func(r *http.Request) (*http.Response, error) {
		if fail {
			return templateResponse(200, "invalid YAML", ""), nil
		}
		if r.Header.Get("If-None-Match") != "" {
			return templateResponse(304, "", ""), nil
		}
		return templateResponse(200, fetchTestTemplate+"# "+r.URL.Path+"\n", `"good"`), nil
	})
	first, err := SaveTemplate("", TemplateInput{Name: "Original", URL: "https://templates.example/original.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSettingsService().Update(variantInput(first.ID)); err != nil {
		t.Fatal(err)
	}
	before, _ := GetTemplateStatus()
	oldDefinition, _ := resolveTemplate(first.ID)
	oldBytes, _ := os.ReadFile(oldDefinition.cachePath())
	fail = true
	if _, err := SaveTemplate(first.ID, TemplateInput{Name: "Broken", URL: "https://templates.example/broken.yaml"}); !errors.Is(err, ErrTemplatePreparation) {
		t.Fatal(err)
	}
	after, _ := GetTemplateStatus()
	if after != before {
		t.Fatalf("failed edit changed active source: %+v -> %+v", before, after)
	}
	data, _ := os.ReadFile(oldDefinition.cachePath())
	if !reflect.DeepEqual(data, oldBytes) {
		t.Fatal("failed edit touched old cache")
	}
	fail = false
	if err := database.GetDB().Exec(`CREATE TRIGGER reject_template_update BEFORE UPDATE ON templates BEGIN SELECT RAISE(FAIL, 'test failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTemplate(first.ID, TemplateInput{Name: "New", URL: "https://templates.example/new.yaml"}); err == nil {
		t.Fatal("DB failure accepted")
	}
	after, _ = GetTemplateStatus()
	if after != before {
		t.Fatal("DB failure switched source")
	}
	if err := database.GetDB().Exec("DROP TRIGGER reject_template_update").Error; err != nil {
		t.Fatal(err)
	}
	changed, err := SaveTemplate(first.ID, TemplateInput{Name: "New", URL: "https://templates.example/new.yaml"})
	if err != nil || changed.ID != first.ID || changed.URL == first.URL {
		t.Fatalf("edit: %+v %v", changed, err)
	}
	newDefinition, _ := resolveTemplate(first.ID)
	if newDefinition.cachePath() == oldDefinition.cachePath() {
		t.Fatal("URL edit reused source cache/ETag")
	}
	content, err := ReadTemplate()
	if err != nil || !strings.Contains(content, "# /new.yaml") {
		t.Fatalf("new active content %q %v", content, err)
	}
}

func TestTemplateCatalogRejectsInvalidSourcesAndBuiltinMutations(t *testing.T) {
	setupTemplateTestDB(t)
	requests := 0
	mockRemoteTemplates(t, func(r *http.Request) (*http.Response, error) {
		requests++
		return templateResponse(200, fetchTestTemplate, ""), nil
	})
	cases := []TemplateInput{
		{Name: "", URL: "https://templates.example/a"},
		{Name: strings.Repeat("名", 101), URL: "https://templates.example/a"},
		{Name: "Upload", URL: "https://templates.example/a", Source: TemplateSourceUpload},
		{Name: "Private", URL: "https://127.0.0.1/a"},
		{Name: "HTTP", URL: "http://templates.example/a"},
	}
	for _, input := range cases {
		if _, err := SaveTemplate("", input); !errors.Is(err, ErrTemplateInput) {
			t.Fatalf("%+v: %v", input, err)
		}
	}
	for _, id := range []string{TemplateVariantGeneral, TemplateVariantAggressive} {
		if _, err := SaveTemplate(id, TemplateInput{Name: "Edit", URL: "https://templates.example/a"}); !errors.Is(err, ErrTemplateReadOnly) {
			t.Fatal(err)
		}
		if err := DeleteTemplate(id); !errors.Is(err, ErrTemplateReadOnly) {
			t.Fatal(err)
		}
	}
	if requests != 0 {
		t.Fatal("rejected input performed download")
	}
	id, alias := TemplateVariantGeneral, TemplateVariantAggressive
	if _, err := NewSettingsService().Update(SettingsInput{ActiveTemplateID: &id, TemplateVariant: &alias}); !errors.Is(err, ErrInvalidTemplateVariant) {
		t.Fatal("conflicting aliases accepted")
	}
	for _, id := range []string{"../general", "remote-../../a", "remote-00000000-0000-0000-0000-000000000000"} {
		if _, err := RefreshTemplateByID(id); !errors.Is(err, ErrTemplateNotFound) {
			t.Fatalf("invalid ID %q %v", id, err)
		}
	}
}
