package service

import (
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

type templateRoundTripper func(*http.Request) (*http.Response, error)

func (f templateRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockTemplateDownloads(t *testing.T, handler templateRoundTripper) {
	t.Helper()
	original := http.DefaultTransport
	http.DefaultTransport = handler
	t.Cleanup(func() { http.DefaultTransport = original })
}

func templateResponse(status int, body, etag string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Etag": []string{etag}}, Body: io.NopCloser(strings.NewReader(body))}
}

func variantInput(variant string) SettingsInput {
	return SettingsInput{UAFilterEnabled: true, UAKeywords: []string{"mihomo"}, UARejectStatus: 404, TemplateVariant: &variant}
}

func variantTemplate(variant string) string {
	dns := "https://dns.alidns.com/dns-query"
	if variant == TemplateVariantAggressive {
		dns = "https://1.1.1.1/dns-query#PROXY"
	}
	return fetchTestTemplate + "dns:\n  nameserver:\n    - '" + dns + "'\nrules:\n  - MATCH,PROXY\n"
}

func assertActiveTemplate(t *testing.T, variant string) {
	t.Helper()
	status, err := GetTemplateStatus()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(templateCachePath(variant))
	if err != nil {
		t.Fatal(err)
	}
	if status.Variant != variant || status.URL != templateURL(variant) || !status.Cached || status.UpdatedAt != info.ModTime().UnixMilli() {
		t.Fatalf("inconsistent status: %+v", status)
	}
	rendered, err := RenderMihomoYAML("https://panel.example", "test-token")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(strings.ReplaceAll(variantTemplate(variant), placeholderApiDomain, "https://panel.example"), placeholderToken, "test-token")
	if rendered != want {
		t.Fatalf("rendered wrong template:\n%s", rendered)
	}
}

func TestTemplateVariantsSwitchRefreshAndRestart(t *testing.T) {
	setupTemplateTestDB(t)
	// A pre-upgrade general cache remains usable without a network request.
	if err := os.WriteFile(TemplateCachePath(), []byte(variantTemplate(TemplateVariantGeneral)), 0o644); err != nil {
		t.Fatal(err)
	}
	defaults, err := NewSettingsService().Get()
	if err != nil || defaults.TemplateVariant != TemplateVariantGeneral {
		t.Fatalf("defaults: %+v %v", defaults, err)
	}
	assertActiveTemplate(t, TemplateVariantGeneral)
	var requests []string
	mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
		variant := TemplateVariantGeneral
		if r.URL.String() == AggressiveTemplateURL {
			variant = TemplateVariantAggressive
		} else if r.URL.String() != DefaultTemplateURL {
			t.Fatalf("untrusted URL: %s", r.URL)
		}
		requests = append(requests, variant+":"+r.Header.Get("If-None-Match"))
		if r.Header.Get("If-None-Match") != "" {
			return templateResponse(304, "", ""), nil
		}
		// Identical ETags across sources must still not skip the first download.
		return templateResponse(200, variantTemplate(variant), `"same-etag"`), nil
	})
	svc := NewSettingsService()
	if _, err := svc.Update(variantInput(TemplateVariantAggressive)); err != nil {
		t.Fatal(err)
	}
	assertActiveTemplate(t, TemplateVariantAggressive)
	result, err := RefreshTemplate()
	if err != nil || result.Changed || result.Variant != TemplateVariantAggressive {
		t.Fatalf("refresh: %+v %v", result, err)
	}
	// Older API clients updating UA settings cannot reset the template selection.
	legacy := variantInput(TemplateVariantGeneral)
	legacy.TemplateVariant = nil
	if settings, err := svc.Update(legacy); err != nil || settings.TemplateVariant != TemplateVariantAggressive {
		t.Fatalf("legacy update: %+v %v", settings, err)
	}
	if _, err := svc.Update(variantInput(TemplateVariantGeneral)); err != nil {
		t.Fatal(err)
	}
	assertActiveTemplate(t, TemplateVariantGeneral)
	if _, err := svc.Update(variantInput(TemplateVariantAggressive)); err != nil {
		t.Fatal(err)
	}
	assertActiveTemplate(t, TemplateVariantAggressive)
	want := []string{"aggressive:", `aggressive:"same-etag"`, "general:", `aggressive:"same-etag"`}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests = %v, want %v", requests, want)
	}
	// Reopen the actual SQLite file and discard in-memory ETags like a restart.
	if err := database.Reset(); err != nil {
		t.Fatal(err)
	}
	templateState.etags = make(map[string]string)
	if err := database.InitDB(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewSettingsService().Get()
	if err != nil || restarted.TemplateVariant != TemplateVariantAggressive {
		t.Fatalf("restart: %+v %v", restarted, err)
	}
	assertActiveTemplate(t, TemplateVariantAggressive)
	if _, err := FetchTemplate(); err != nil {
		t.Fatal(err)
	}
	if requests[len(requests)-1] != "aggressive:" {
		t.Fatalf("restart fetched wrong source: %v", requests)
	}
}

func TestTemplateSwitchFailurePreservesSettingsAndBothCaches(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{name: "HTTP error", status: 503},
		{name: "network error", err: errors.New("offline")},
		{name: "empty", status: 200},
		{name: "missing placeholder", status: 200, body: "mixed-port: 7890\n"},
		{name: "invalid YAML", status: 200, body: "# __API_DOMAIN__ __TOKEN__\nx: [unclosed"},
		{name: "comment-only placeholders", status: 200, body: "# __API_DOMAIN__ __TOKEN__\nmixed-port: 7890\n"},
		{name: "too large", status: 200, body: strings.Repeat("x", templateMaxBytes+1)},
		{name: "unsolicited 304", status: 304},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupTemplateTestDB(t)
			before, err := NewSettingsService().Get()
			if err != nil {
				t.Fatal(err)
			}
			cacheTimes := make(map[string]int64)
			for _, variant := range []string{TemplateVariantGeneral, TemplateVariantAggressive} {
				if err := os.WriteFile(templateCachePath(variant), []byte(variantTemplate(variant)), 0o644); err != nil {
					t.Fatal(err)
				}
				info, _ := os.Stat(templateCachePath(variant))
				cacheTimes[variant] = info.ModTime().UnixNano()
			}
			mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != AggressiveTemplateURL {
					t.Errorf("wrong source %s", r.URL)
				}
				if tc.err != nil {
					return nil, tc.err
				}
				return templateResponse(tc.status, tc.body, `"bad"`), nil
			})
			if _, err := NewSettingsService().Update(variantInput(TemplateVariantAggressive)); err == nil {
				t.Fatal("invalid switch succeeded")
			}
			after, err := NewSettingsService().Get()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("settings changed on failure: %+v -> %+v (%v)", before, after, err)
			}
			assertActiveTemplate(t, TemplateVariantGeneral)
			for variant, mtime := range cacheTimes {
				data, info, err := readTemplateCache(templateCachePath(variant))
				if err != nil || string(data) != variantTemplate(variant) || info.ModTime().UnixNano() != mtime {
					t.Fatalf("%s cache damaged: %v", variant, err)
				}
			}
		})
	}
}

func TestSelectedTemplateNeverFallsBackToOtherVersion(t *testing.T) {
	setupTemplateTestDB(t)
	if _, err := NewSettingsService().Get(); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(&model.Settings{}).Where("id = 1").Update("template_variant", TemplateVariantAggressive).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(TemplateCachePath(), []byte(variantTemplate(TemplateVariantGeneral)), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := GetTemplateStatus()
	if err != nil || status.Variant != TemplateVariantAggressive || status.URL != AggressiveTemplateURL || status.Cached || status.UpdatedAt != 0 {
		t.Fatalf("status: %+v %v", status, err)
	}
	if _, err := RenderMihomoYAML("https://panel.example", "test-token"); err == nil {
		t.Fatal("fell back to general cache")
	}
}

func TestTemplateSaveAndRefreshSerializeWithoutBlockingCurrentRender(t *testing.T) {
	setupTemplateTestDB(t)
	if _, err := NewSettingsService().Get(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(TemplateCachePath(), []byte(variantTemplate(TemplateVariantGeneral)), 0o644); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == DefaultTemplateURL {
			close(started)
			<-release
			return templateResponse(200, variantTemplate(TemplateVariantGeneral), `"general"`), nil
		}
		if r.URL.String() != AggressiveTemplateURL {
			t.Errorf("unexpected source: %s", r.URL)
		}
		return templateResponse(200, variantTemplate(TemplateVariantAggressive), `"aggressive"`), nil
	})
	type refreshOutcome struct {
		result TemplateRefreshResult
		err    error
	}
	refreshed := make(chan refreshOutcome, 1)
	go func() { result, err := RefreshTemplate(); refreshed <- refreshOutcome{result, err} }()
	<-started
	switched := make(chan error, 1)
	go func() {
		_, err := NewSettingsService().Update(variantInput(TemplateVariantAggressive))
		switched <- err
	}()
	assertActiveTemplate(t, TemplateVariantGeneral)
	releaseOnce.Do(func() { close(release) })
	outcome := <-refreshed
	if outcome.err != nil || outcome.result.Variant != TemplateVariantGeneral || outcome.result.URL != DefaultTemplateURL {
		t.Fatalf("refresh lost its source snapshot: %+v", outcome)
	}
	if err := <-switched; err != nil {
		t.Fatal(err)
	}
	assertActiveTemplate(t, TemplateVariantAggressive)
}

func TestTemplateVariantRejectsUntrustedInputs(t *testing.T) {
	setupTemplateTestDB(t)
	mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
		t.Errorf("invalid input caused download: %s", r.URL)
		return nil, errors.New("unexpected download")
	})
	for _, variant := range []string{"", "unknown", "https://example.com/template.yaml", "../aggressive", "GENERAL"} {
		if _, err := NewSettingsService().Update(variantInput(variant)); !errors.Is(err, ErrInvalidTemplateVariant) {
			t.Errorf("%q: %v", variant, err)
		}
	}
}

func TestRefreshFailureKeepsSelectedVariantCacheAndETag(t *testing.T) {
	setupTemplateTestDB(t)
	fail := false
	mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != AggressiveTemplateURL {
			t.Errorf("wrong source: %s", r.URL)
		}
		if fail {
			return templateResponse(200, "invalid template", `"bad"`), nil
		}
		return templateResponse(200, variantTemplate(TemplateVariantAggressive), `"good"`), nil
	})
	if _, err := NewSettingsService().Update(variantInput(TemplateVariantAggressive)); err != nil {
		t.Fatal(err)
	}
	before, err := GetTemplateStatus()
	if err != nil {
		t.Fatal(err)
	}
	fail = true
	if _, err := RefreshTemplate(); err == nil {
		t.Fatal("invalid refresh succeeded")
	}
	after, err := GetTemplateStatus()
	if err != nil || before != after {
		t.Fatalf("status changed after failed refresh: %+v -> %+v (%v)", before, after, err)
	}
	if templateState.etags[templateCachePath(TemplateVariantAggressive)] != `"good"` {
		t.Fatal("failed refresh corrupted ETag")
	}
	assertActiveTemplate(t, TemplateVariantAggressive)
}

func TestCorruptTemplateCacheForcesUnconditionalRefresh(t *testing.T) {
	setupTemplateTestDB(t)
	if err := os.WriteFile(TemplateCachePath(), []byte("invalid cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	templateState.etags[TemplateCachePath()] = `"old"`
	status, err := GetTemplateStatus()
	if err != nil || status.Cached || status.UpdatedAt != 0 {
		t.Fatalf("corrupt cache reported as usable: %+v %v", status, err)
	}
	if _, err := ReadTemplate(); err == nil {
		t.Fatal("read corrupt template")
	}
	mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("If-None-Match"); got != "" {
			t.Errorf("corrupt cache sent conditional request: %q", got)
		}
		return templateResponse(200, variantTemplate(TemplateVariantGeneral), `"new"`), nil
	})
	if _, err := FetchTemplate(); err != nil {
		t.Fatal(err)
	}
	assertActiveTemplate(t, TemplateVariantGeneral)
}

func TestTemplateSwitchDatabaseFailureKeepsCurrentSelection(t *testing.T) {
	setupTemplateTestDB(t)
	before, err := NewSettingsService().Get()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(TemplateCachePath(), []byte(variantTemplate(TemplateVariantGeneral)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Exec(`CREATE TRIGGER reject_settings_update BEFORE UPDATE ON settings BEGIN SELECT RAISE(FAIL, 'test database failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
		return templateResponse(200, variantTemplate(TemplateVariantAggressive), `"good"`), nil
	})
	if _, err := NewSettingsService().Update(variantInput(TemplateVariantAggressive)); err == nil {
		t.Fatal("expected database failure")
	}
	after, err := NewSettingsService().Get()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("database failure activated target: %+v %v", after, err)
	}
	assertActiveTemplate(t, TemplateVariantGeneral)
}
