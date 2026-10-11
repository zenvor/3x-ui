package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
)

func TestSuccessfulCheckMetadataSupportsMaximumEscapedURL(t *testing.T) {
	setupTemplateTestDB(t)
	const prefix = "https://templates.example/template.yaml?q="
	url := prefix + strings.Repeat("&", 2048-len(prefix))
	response := 200
	mockRemoteTemplates(t, func(r *http.Request) (*http.Response, error) {
		return templateResponse(response, fetchTestTemplate, `"etag"`), nil
	})
	record, err := SaveTemplate("", TemplateInput{Name: "Escaped URL", URL: url})
	if err != nil || !record.Cached || record.CheckedAt <= 0 || record.CheckResult != "updated" {
		t.Fatalf("successful check was lost: %+v %v", record, err)
	}
	definition, err := resolveTemplate(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile(definition.cachePath() + ".check.json")
	if err != nil || len(metadata) <= 4096 {
		t.Fatalf("test must exceed old metadata limit: %d %v", len(metadata), err)
	}
	response = 304
	refreshed, err := RefreshTemplateByID(record.ID)
	if err != nil || refreshed.Changed || refreshed.CheckedAt <= 0 || refreshed.CheckResult != "unchanged" {
		t.Fatalf("304 check was lost: %+v %v", refreshed, err)
	}
	if err := database.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(); err != nil {
		t.Fatal(err)
	}
	restarted, err := ListTemplates()
	if err != nil || len(restarted) != 3 || restarted[2].TemplateStatus != refreshed.TemplateStatus {
		t.Fatalf("restart lost check history: %+v %v", restarted, err)
	}
}

func TestCheckMetadataRejectsOversizedAndTrailingContent(t *testing.T) {
	setupTemplateTestDB(t)
	seedTemplateCheck(t, TemplateVariantGeneral, DefaultTemplateURL, fetchTestTemplate, 1000, "updated")
	path := TemplateCachePath() + ".check.json"
	valid, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		string(valid) + strings.Repeat(" ", (16<<10)+1),
		string(valid) + "garbage",
	} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readTemplateCheck(path[:len(path)-len(".check.json")], DefaultTemplateURL, []byte(fetchTestTemplate)); err == nil {
			t.Fatal("invalid metadata was accepted")
		}
	}
}

func seedTemplateCheck(t *testing.T, variant, url, content string, checkedAt int64, result string) {
	t.Helper()
	data, err := json.Marshal(templateCheck{URL: url, CacheSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))), CheckedAt: checkedAt, CheckResult: result})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(templateCachePath(variant)+".check.json", data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSuccessfulCheckTimesDistinguish304SameBody200AndChangedBody(t *testing.T) {
	setupTemplateTestDB(t)
	responseStatus, responseBody := 200, fetchTestTemplate
	mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
		return templateResponse(responseStatus, responseBody, `"etag"`), nil
	})
	first, err := RefreshTemplate()
	if err != nil || !first.Changed || first.CheckedAt <= 0 || first.CheckResult != "updated" {
		t.Fatalf("initial check: %+v %v", first, err)
	}
	// Use an old cache/record time without relying on sleeps or clock precision.
	oldTime := time.Unix(1234567890, 0)
	if err := os.Chtimes(TemplateCachePath(), oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	for _, response := range []int{304, 200} {
		responseStatus = response
		seedTemplateCheck(t, TemplateVariantGeneral, DefaultTemplateURL, fetchTestTemplate, 1000, "updated")
		startedAt := time.Now().UnixMilli()
		checked, err := RefreshTemplate()
		if err != nil || checked.Changed || checked.CheckResult != "unchanged" || checked.CheckedAt < startedAt || checked.UpdatedAt != oldTime.UnixMilli() {
			t.Fatalf("HTTP %d check: %+v %v", response, checked, err)
		}
		content, info, err := readTemplateCache(TemplateCachePath())
		if err != nil || string(content) != fetchTestTemplate || !info.ModTime().Equal(oldTime) {
			t.Fatalf("HTTP %d touched cache: %v", response, err)
		}
		// A failed request preserves both successful-check metadata and YAML time.
		before := checked.TemplateStatus
		responseStatus = 503
		if _, err := RefreshTemplate(); err == nil {
			t.Fatal("failed request reported success")
		}
		after, err := GetTemplateStatus()
		if err != nil || after != before {
			t.Fatalf("failed request altered times: %+v -> %+v (%v)", before, after, err)
		}
	}
	responseStatus, responseBody = 200, fetchTestTemplate+"mixed-port: 7891\n"
	updated, err := RefreshTemplate()
	if err != nil || !updated.Changed || updated.CheckResult != "updated" || updated.CheckedAt <= 0 || updated.UpdatedAt <= oldTime.UnixMilli() {
		t.Fatalf("changed body check: %+v %v", updated, err)
	}
}

func TestCheckRecordsAreIndependentAndSurviveRestart(t *testing.T) {
	setupTemplateTestDB(t)
	mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
		content := variantTemplate(TemplateVariantGeneral)
		if r.URL.String() == AggressiveTemplateURL {
			content = variantTemplate(TemplateVariantAggressive)
		}
		return templateResponse(200, content, ""), nil
	})
	if _, err := RefreshTemplate(); err != nil {
		t.Fatal(err)
	}
	seedTemplateCheck(t, TemplateVariantGeneral, DefaultTemplateURL, variantTemplate(TemplateVariantGeneral), 1000, "updated")
	if _, err := NewSettingsService().Update(variantInput(TemplateVariantAggressive)); err != nil {
		t.Fatal(err)
	}
	aggressive, err := GetTemplateStatus()
	if err != nil || aggressive.CheckedAt <= 1000 || aggressive.CheckResult != "updated" {
		t.Fatalf("aggressive status: %+v %v", aggressive, err)
	}
	if general := templateStatusForVariant(TemplateVariantGeneral); general.CheckedAt != 1000 {
		t.Fatalf("aggressive check changed general history: %+v", general)
	}
	if err := database.Reset(); err != nil {
		t.Fatal(err)
	}
	templateState.etags = make(map[string]string)
	if err := database.InitDB(); err != nil {
		t.Fatal(err)
	}
	restored, err := GetTemplateStatus()
	if err != nil || restored != aggressive {
		t.Fatalf("restart lost successful check: %+v -> %+v (%v)", aggressive, restored, err)
	}
	// The first fetch after restart has no ETag but identical bytes still do
	// not replace the cache or reset its update time.
	rechecked, err := RefreshTemplate()
	if err != nil || rechecked.Changed || rechecked.UpdatedAt != restored.UpdatedAt || rechecked.CheckResult != "unchanged" {
		t.Fatalf("post-restart check: %+v %v", rechecked, err)
	}
}

func TestLegacyCorruptOrMismatchedCheckMetadataRemainsUnknown(t *testing.T) {
	setupTemplateTestDB(t)
	if err := os.WriteFile(TemplateCachePath(), []byte(fetchTestTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	assertUnknown := func() {
		t.Helper()
		status, err := GetTemplateStatus()
		if err != nil || !status.Cached || status.UpdatedAt == 0 || status.CheckedAt != 0 || status.CheckResult != "" {
			t.Fatalf("invented check history: %+v %v", status, err)
		}
	}
	assertUnknown()
	if err := os.WriteFile(TemplateCachePath()+".check.json", []byte("broken JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertUnknown()
	seedTemplateCheck(t, TemplateVariantGeneral, AggressiveTemplateURL, fetchTestTemplate, 1000, "updated")
	assertUnknown()
	seedTemplateCheck(t, TemplateVariantGeneral, DefaultTemplateURL, "different content", 1000, "updated")
	assertUnknown()
	seedTemplateCheck(t, TemplateVariantGeneral, DefaultTemplateURL, fetchTestTemplate, 1000, "updated")
	status, err := GetTemplateStatus()
	if err != nil || status.CheckedAt != 1000 {
		t.Fatalf("valid record ignored: %+v %v", status, err)
	}
	if err := os.Remove(TemplateCachePath()); err != nil {
		t.Fatal(err)
	}
	status, err = GetTemplateStatus()
	if err != nil || status.Cached || status.UpdatedAt != 0 || status.CheckedAt != 0 || status.CheckResult != "" {
		t.Fatalf("record with no cache leaked: %+v %v", status, err)
	}
}

func TestFailedCheckRecordWriteDoesNotClaimSuccessOrTouchUnchangedCache(t *testing.T) {
	setupTemplateTestDB(t)
	if err := os.WriteFile(TemplateCachePath(), []byte(fetchTestTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := GetTemplateStatus()
	if err != nil {
		t.Fatal(err)
	}
	// Renaming the metadata file over a directory must fail on all platforms.
	if err := os.Mkdir(TemplateCachePath()+".check.json", 0o755); err != nil {
		t.Fatal(err)
	}
	mockTemplateDownloads(t, func(r *http.Request) (*http.Response, error) {
		return templateResponse(200, fetchTestTemplate, `"new"`), nil
	})
	if _, err := RefreshTemplate(); err == nil {
		t.Fatal("metadata write failure claimed success")
	}
	after, err := GetTemplateStatus()
	if err != nil || after != before {
		t.Fatalf("failed record write changed status: %+v %v", after, err)
	}
	if templateState.etags[TemplateCachePath()] != "" {
		t.Fatal("failed record write advanced ETag")
	}
}
