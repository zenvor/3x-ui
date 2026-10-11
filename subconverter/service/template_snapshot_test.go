package service

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/subconverter/database"
	"github.com/mhsanaei/3x-ui/v3/subconverter/model"
	"gorm.io/gorm"
)

func TestTemplateReadersSurviveActiveSwitchAndDelete(t *testing.T) {
	for _, reader := range []string{"settings", "content", "status"} {
		t.Run(reader, func(t *testing.T) {
			setupTemplateTestDB(t)
			mockRemoteTemplates(t, func(r *http.Request) (*http.Response, error) {
				return templateResponse(200, fetchTestTemplate+"# "+r.URL.Path+"\n", ""), nil
			})
			first, err := SaveTemplate("", TemplateInput{Name: "A", URL: "https://templates.example/a.yaml"})
			if err != nil {
				t.Fatal(err)
			}
			second, err := SaveTemplate("", TemplateInput{Name: "B", URL: "https://templates.example/b.yaml"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := NewSettingsService().Update(variantInput(first.ID)); err != nil {
				t.Fatal(err)
			}
			definition, err := resolveTemplate(first.ID)
			if err != nil {
				t.Fatal(err)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			var paused atomic.Bool
			callback := "test:pause-template-reader"
			db := database.GetDB()
			if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Table == "settings" && paused.CompareAndSwap(false, true) {
					close(entered)
					<-release
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
			type outcome struct {
				id      string
				content string
				err     error
			}
			done := make(chan outcome, 1)
			go func() {
				var result outcome
				switch reader {
				case "settings":
					settings, err := NewSettingsService().Get()
					result.err = err
					if settings != nil {
						result.id = settings.TemplateVariant
						if !IsUserAgentAllowed("mihomo", settings) {
							result.content = "UA policy was lost"
						}
					}
				case "content":
					result.content, result.err = ReadTemplate()
				case "status":
					status, err := GetTemplateStatus()
					result.id, result.err = status.ID, err
					if !status.Cached {
						result.content = "cache was lost"
					}
				}
				done <- result
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("reader did not reach snapshot boundary")
			}
			// The query has read A, but has not returned to the caller. Neither
			// mutation may wait for the reader or remove its retained cache.
			if _, err := NewSettingsService().Update(variantInput(second.ID)); err != nil {
				t.Fatal(err)
			}
			if err := DeleteTemplate(first.ID); err != nil {
				t.Fatal(err)
			}
			unblock()
			select {
			case result := <-done:
				if result.err != nil || (reader != "content" && result.id != first.ID) {
					t.Fatalf("in-flight %s failed: %+v", reader, result)
				}
				if reader == "content" && !strings.Contains(result.content, "# /a.yaml") {
					t.Fatalf("reader did not retain source A: %+v", result)
				}
				if reader != "content" && result.content != "" {
					t.Fatal(result.content)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("reader did not finish")
			}
			if _, err := os.Stat(definition.cachePath()); err != nil {
				t.Fatal("in-flight cache removed:", err)
			}
			current, err := GetTemplateStatus()
			if err != nil || current.ID != second.ID || !current.Cached {
				t.Fatalf("new reader did not see B: %+v %v", current, err)
			}
		})
	}
}

func TestUASettingsDoNotDependOnTemplateCatalog(t *testing.T) {
	setupTemplateTestDB(t)
	if _, err := NewSettingsService().Get(); err != nil {
		t.Fatal(err)
	}
	const missing = "remote-11111111-1111-4111-8111-111111111111"
	if err := database.GetDB().Model(&model.Settings{}).Where("id = 1").Update("template_variant", missing).Error; err != nil {
		t.Fatal(err)
	}
	settings, err := NewSettingsService().Get()
	if err != nil || !IsUserAgentAllowed("mihomo", settings) || IsUserAgentAllowed("curl", settings) {
		t.Fatalf("template catalog broke UA policy: %+v %v", settings, err)
	}
	if _, err := ReadTemplate(); err == nil {
		t.Fatal("missing active template must still fail rendering")
	}
}

func TestTemplateSnapshotInitializesDefaultSelection(t *testing.T) {
	setupTemplateTestDB(t)
	if err := os.WriteFile(TemplateCachePath(), []byte(fetchTestTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := GetTemplateStatus()
	if err != nil || status.ID != TemplateVariantGeneral || !status.Cached {
		t.Fatalf("initial status: %+v %v", status, err)
	}
	content, err := ReadTemplate()
	if err != nil || content != fetchTestTemplate {
		t.Fatalf("initial content: %q %v", content, err)
	}
}
