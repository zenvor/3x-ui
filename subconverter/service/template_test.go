package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const fetchTestTemplate = "proxy-providers:\n  main:\n    url: '__API_DOMAIN__/feed/__TOKEN__/nodes'\n"

func newTemplateServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server, filepath.Join(t.TempDir(), "mihomo-template.yaml")
}

func TestFetchTemplateWritesCacheAndReturnsETag(t *testing.T) {
	server, cachePath := newTemplateServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("If-None-Match"); got != "" {
			t.Errorf("first fetch should not send If-None-Match, got %q", got)
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(fetchTestTemplate))
	})

	changed, etag, err := fetchTemplate(context.Background(), server.Client(), server.URL, cachePath, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true after a 200 fetch")
	}
	if etag != `"v1"` {
		t.Fatalf("etag = %q, want %q", etag, `"v1"`)
	}
	cached, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	if string(cached) != fetchTestTemplate {
		t.Fatalf("cache content = %q, want the server body", cached)
	}
}

func TestFetchTemplate304KeepsExistingCache(t *testing.T) {
	server, cachePath := newTemplateServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != `"v1"` {
			t.Errorf("If-None-Match = %q, want %q", r.Header.Get("If-None-Match"), `"v1"`)
		}
		w.WriteHeader(http.StatusNotModified)
	})
	if err := os.WriteFile(cachePath, []byte(fetchTestTemplate), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	changed, etag, err := fetchTemplate(context.Background(), server.Client(), server.URL, cachePath, `"v1"`)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if changed {
		t.Fatal("changed = true, want false on 304")
	}
	if etag != `"v1"` {
		t.Fatalf("etag = %q, want it preserved as %q", etag, `"v1"`)
	}
	cached, _ := os.ReadFile(cachePath)
	if string(cached) != fetchTestTemplate {
		t.Fatal("304 fetch must not touch the cached file")
	}
}

func TestFetchTemplateRejectsContentWithoutPlaceholders(t *testing.T) {
	server, cachePath := newTemplateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"bad"`)
		_, _ = w.Write([]byte("mixed-port: 7890\n"))
	})
	if err := os.WriteFile(cachePath, []byte(fetchTestTemplate), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	changed, etag, err := fetchTemplate(context.Background(), server.Client(), server.URL, cachePath, `"v1"`)
	if err == nil || !strings.Contains(err.Error(), "missing placeholder") {
		t.Fatalf("err = %v, want a missing-placeholder rejection", err)
	}
	if changed {
		t.Fatal("changed = true, want false for a rejected download")
	}
	if etag != `"v1"` {
		t.Fatalf("etag = %q, want it unchanged after rejection", etag)
	}
	cached, _ := os.ReadFile(cachePath)
	if string(cached) != fetchTestTemplate {
		t.Fatal("rejected download must not overwrite the previous cache")
	}
}

func TestFetchTemplateRejectsEmptyBody(t *testing.T) {
	server, cachePath := newTemplateServer(t, func(w http.ResponseWriter, r *http.Request) {})

	if _, _, err := fetchTemplate(context.Background(), server.Client(), server.URL, cachePath, ""); err == nil ||
		!strings.Contains(err.Error(), "template is empty") {
		t.Fatalf("err = %v, want an empty-template rejection", err)
	}
}

func TestFetchTemplateRejectsBrokenYAMLWithPlaceholders(t *testing.T) {
	// Placeholders present (in a comment is enough for the string check) but
	// the YAML itself does not parse: must not replace the known-good cache.
	broken := "# __API_DOMAIN__ __TOKEN__\nproxy-providers:\n  main: [unclosed\n"
	server, cachePath := newTemplateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"bad"`)
		_, _ = w.Write([]byte(broken))
	})
	if err := os.WriteFile(cachePath, []byte(fetchTestTemplate), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	changed, etag, err := fetchTemplate(context.Background(), server.Client(), server.URL, cachePath, `"v1"`)
	if err == nil || !strings.Contains(err.Error(), "not valid YAML") {
		t.Fatalf("err = %v, want a YAML parse rejection", err)
	}
	if changed || etag != `"v1"` {
		t.Fatalf("changed=%v etag=%q, want no cache write and preserved etag", changed, etag)
	}
	cached, _ := os.ReadFile(cachePath)
	if string(cached) != fetchTestTemplate {
		t.Fatal("rejected download must not overwrite the previous cache")
	}
}

func TestFetchTemplateUnexpectedStatus(t *testing.T) {
	server, cachePath := newTemplateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, _, err := fetchTemplate(context.Background(), server.Client(), server.URL, cachePath, ""); err == nil ||
		!strings.Contains(err.Error(), "unexpected status 500") {
		t.Fatalf("err = %v, want an unexpected-status error", err)
	}
}

func TestReadTemplateMissesWithoutCache(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())

	if _, err := ReadTemplate(); err == nil {
		t.Fatal("expected error when the cache file does not exist")
	}
}

func TestFetchTemplateStateForcesFullFetchWhenCacheMissing(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	var conditionalRequests atomic.Int32
	server, _ := newTemplateServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "" {
			conditionalRequests.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(fetchTestTemplate))
	})

	// Simulate a panel that fetched successfully before but lost its cache file.
	templateState.etag = `"v1"`
	t.Cleanup(func() { templateState.etag = "" })

	changed, err := fetchTemplateState(server.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want a forced full fetch when the cache file is gone")
	}
	if conditionalRequests.Load() != 0 {
		t.Fatalf("server saw %d conditional requests, want none once the cache is missing", conditionalRequests.Load())
	}
	if _, err := os.Stat(TemplateCachePath()); err != nil {
		t.Fatalf("cache file should have been restored: %v", err)
	}

	// Second fetch: cache exists and the ETag is current, so a 304 is correct now.
	changed, err = fetchTemplateState(server.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if changed {
		t.Fatal("changed = true, want false on 304 with an intact cache")
	}
	if conditionalRequests.Load() != 1 {
		t.Fatalf("server saw %d conditional requests, want 1", conditionalRequests.Load())
	}
}

func TestGetTemplateStatusReflectsCacheFile(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())

	if status := GetTemplateStatus(); status.Cached || status.UpdatedAt != 0 {
		t.Fatalf("status = %+v, want uncached", status)
	}
	if err := os.WriteFile(TemplateCachePath(), []byte(fetchTestTemplate), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	status := GetTemplateStatus()
	if !status.Cached || status.UpdatedAt <= 0 {
		t.Fatalf("status = %+v, want cached with a positive mtime", status)
	}
	if status.URL != DefaultTemplateURL {
		t.Fatalf("status.URL = %q, want %q", status.URL, DefaultTemplateURL)
	}
}
