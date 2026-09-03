package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const (
	// DefaultTemplateURL is the raw address of the mihomo-config repository's
	// template; the panel refreshes its on-disk cache from here.
	DefaultTemplateURL      = "https://raw.githubusercontent.com/zenvor/mihomo-config/main/mihomo.yaml"
	templateCacheFileName   = "mihomo-template.yaml"
	templateRefreshInterval = 10 * time.Minute
	templateFetchTimeout    = 15 * time.Second
	templateMaxBytes        = 1 << 20
)

// templateState keeps the last ETag in memory only: losing it on restart just
// costs one unconditional re-download of a few KB.
var templateState = struct {
	sync.Mutex
	etag string
}{}

// templateRefreshOnce guards StartTemplateRefresh against SIGHUP reloads.
var templateRefreshOnce sync.Once

// TemplateStatus is the API-facing view of the cached Mihomo template.
type TemplateStatus struct {
	URL       string `json:"url"`
	Cached    bool   `json:"cached"`
	UpdatedAt int64  `json:"updatedAt"` // cache file mtime, unix ms; 0 when not cached
}

// TemplateRefreshResult pairs the post-refresh status with whether the pull
// actually replaced the cached file, so the UI can say "already up to date".
type TemplateRefreshResult struct {
	TemplateStatus
	Changed bool `json:"changed"`
}

// TemplateCachePath resolves the on-disk cache location next to subconverter.db.
func TemplateCachePath() string {
	return filepath.Join(config.GetDBFolderPath(), templateCacheFileName)
}

func GetTemplateStatus() TemplateStatus {
	status := TemplateStatus{URL: DefaultTemplateURL}
	if info, err := os.Stat(TemplateCachePath()); err == nil {
		status.Cached = true
		status.UpdatedAt = info.ModTime().UnixMilli()
	}
	return status
}

// ReadTemplate returns the cached template for rendering.
func ReadTemplate() (string, error) {
	data, err := os.ReadFile(TemplateCachePath())
	if err != nil {
		return "", fmt.Errorf("read template cache: %w", err)
	}
	return string(data), nil
}

// FetchTemplate refreshes the disk cache from DefaultTemplateURL, serializing
// against concurrent manual/scheduled fetches. Returns whether the cache changed.
func FetchTemplate() (bool, error) {
	return fetchTemplateState(DefaultTemplateURL)
}

// fetchTemplateState is FetchTemplate with the URL as a parameter so tests can
// point it at an httptest server instead of GitHub.
func fetchTemplateState(url string) (bool, error) {
	templateState.Lock()
	defer templateState.Unlock()
	etag := templateState.etag
	// A cache file that vanished underneath us must force a full download;
	// otherwise the server keeps answering 304 and the cache never comes back.
	if _, err := os.Stat(TemplateCachePath()); err != nil {
		etag = ""
	}
	changed, etag, err := fetchTemplate(context.Background(), &http.Client{Timeout: templateFetchTimeout}, url, TemplateCachePath(), etag)
	if err != nil {
		return false, err
	}
	templateState.etag = etag
	return changed, nil
}

// fetchTemplate is the pure core of FetchTemplate: conditional GET, validate,
// atomic replace. ETag goes in and out so tests stay free of package state.
func fetchTemplate(ctx context.Context, client *http.Client, url, cachePath, etag string) (bool, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, etag, fmt.Errorf("build template request: %w", err)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, etag, fmt.Errorf("download template: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return false, etag, nil
	case http.StatusOK:
	default:
		return false, etag, fmt.Errorf("download template: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, templateMaxBytes+1))
	if err != nil {
		return false, etag, fmt.Errorf("read template body: %w", err)
	}
	if len(body) > templateMaxBytes {
		return false, etag, fmt.Errorf("template exceeds %d bytes", templateMaxBytes)
	}
	if err := validateTemplate(string(body)); err != nil {
		return false, etag, fmt.Errorf("template rejected: %w", err)
	}
	if err := atomicWriteFile(cachePath, body); err != nil {
		return false, etag, fmt.Errorf("write template cache: %w", err)
	}
	return true, resp.Header.Get("ETag"), nil
}

// validateTemplate rejects downloads that would break every rendered
// subscription: empty, not a parseable YAML mapping, or missing a placeholder.
func validateTemplate(content string) error {
	if strings.TrimSpace(content) == "" {
		return errors.New("template is empty")
	}
	for _, placeholder := range []string{placeholderApiDomain, placeholderToken} {
		if !strings.Contains(content, placeholder) {
			return fmt.Errorf("missing placeholder %s", placeholder)
		}
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return fmt.Errorf("template is not valid YAML: %w", err)
	}
	return nil
}

// atomicWriteFile writes via a sibling temp file + rename so concurrent
// readers of the cache never see a truncated template.
func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// StartTemplateRefresh starts the background fetch+ticker once per process:
// SIGHUP panel reloads re-run RegisterRoutes, and a second ticker would leak.
func StartTemplateRefresh() {
	templateRefreshOnce.Do(func() {
		go func() {
			if _, err := FetchTemplate(); err != nil {
				logger.Warning("subconverter template initial fetch failed:", err)
			}
			ticker := time.NewTicker(templateRefreshInterval)
			defer ticker.Stop()
			for range ticker.C {
				if _, err := FetchTemplate(); err != nil {
					logger.Warning("subconverter template refresh failed:", err)
				}
			}
		}()
	})
}
