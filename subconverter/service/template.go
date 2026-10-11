package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
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
	TemplateVariantGeneral    = "general"
	TemplateVariantAggressive = "aggressive"
	// Built-in URLs remain immutable catalog entries.
	DefaultTemplateURL      = "https://raw.githubusercontent.com/zenvor/mihomo-config/main/mihomo.yaml"
	AggressiveTemplateURL   = "https://raw.githubusercontent.com/zenvor/mihomo-config/main/mihomo-aggressive.yaml"
	templateCacheFileName   = "mihomo-template.yaml"
	templateRefreshInterval = 10 * time.Minute
	templateFetchTimeout    = 15 * time.Second
	templateMaxBytes        = 1 << 20
	// A 2048-byte URL can expand sixfold when JSON escapes HTML characters.
	templateCheckMaxBytes = 16 << 10
)

var ErrInvalidTemplateVariant = errors.New("invalid Mihomo template variant")

// ETags are keyed by cache path (including version and DB folder). Losing
// them on restart just costs one unconditional re-download of a few KB.
var templateState = struct {
	sync.Mutex
	etags map[string]string
}{etags: make(map[string]string)}

// templateRefreshOnce guards StartTemplateRefresh against SIGHUP reloads.
var templateRefreshOnce sync.Once

// TemplateStatus is the API-facing view of the cached Mihomo template.
type TemplateStatus struct {
	Variant     string `json:"variant"` // legacy API alias of ID
	ID          string `json:"id"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	URL         string `json:"url"`
	Cached      bool   `json:"cached"`
	UpdatedAt   int64  `json:"updatedAt"`             // cache file mtime, unix ms; 0 when not cached
	CheckedAt   int64  `json:"checkedAt"`             // last successful check, including 304; 0 when unknown
	CheckResult string `json:"checkResult,omitempty"` // updated or unchanged; empty when unknown
}

// Successful checks survive restarts and belong to an exact source + cached
// content. Keep metadata separate so a 304 never touches the YAML's mtime.
type templateCheck struct {
	URL         string `json:"url"`
	CacheSHA256 string `json:"cacheSHA256"`
	CheckedAt   int64  `json:"checkedAt"`
	CheckResult string `json:"checkResult"`
}

// TemplateRefreshResult pairs the post-refresh status with whether the pull
// actually replaced the cached file, so the UI can say "already up to date".
type TemplateRefreshResult struct {
	TemplateStatus
	Changed bool `json:"changed"`
}

// TemplateCachePath retains the legacy general cache location. Aggressive
// always uses a separate file; neither version falls back to the other.
func TemplateCachePath() string {
	return templateCachePath(TemplateVariantGeneral)
}

func templateCachePath(variant string) string {
	name := templateCacheFileName
	if variant == TemplateVariantAggressive {
		name = "mihomo-template-aggressive.yaml"
	}
	return filepath.Join(config.GetDBFolderPath(), name)
}

func templateURL(variant string) string {
	if variant == TemplateVariantAggressive {
		return AggressiveTemplateURL
	}
	return DefaultTemplateURL
}

func normalizeTemplateVariant(variant string) string {
	if variant == "" {
		return TemplateVariantGeneral
	}
	return variant
}

func selectedTemplateVariant() (string, error) {
	settings, err := NewSettingsService().Get()
	if err != nil {
		return "", err
	}
	return settings.TemplateVariant, nil
}

func GetTemplateStatus() (TemplateStatus, error) {
	d, err := selectedTemplateDefinition()
	if err != nil {
		return TemplateStatus{}, err
	}
	return templateStatusForDefinition(d), nil
}

func templateStatusForVariant(variant string) TemplateStatus {
	d, err := resolveTemplate(variant)
	if err != nil {
		return TemplateStatus{Variant: variant}
	}
	return templateStatusForDefinition(d)
}

func templateStatusForDefinition(d templateDefinition) TemplateStatus {
	status := TemplateStatus{Variant: d.ID, ID: d.ID, Name: d.Name, Source: d.Source, URL: d.URL}
	// Read and stat the same opened file, even if a refresh renames the path.
	cachePath, err := d.cachePath()
	if err != nil {
		return status
	}
	if data, info, err := readTemplateCache(cachePath); err == nil {
		status.Cached = true
		status.UpdatedAt = info.ModTime().UnixMilli()
		if check, err := readTemplateCheck(cachePath, status.URL, data); err == nil {
			status.CheckedAt = check.CheckedAt
			status.CheckResult = check.CheckResult
		}
	}
	return status
}

func readTemplateCheck(cachePath, url string, cached []byte) (templateCheck, error) {
	file, err := os.Open(cachePath + ".check.json")
	if err != nil {
		return templateCheck{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, templateCheckMaxBytes+1))
	if err != nil {
		return templateCheck{}, err
	}
	if len(data) > templateCheckMaxBytes {
		return templateCheck{}, errors.New("template check metadata exceeds size limit")
	}
	var check templateCheck
	if err := json.Unmarshal(data, &check); err != nil {
		return templateCheck{}, err
	}
	if check.URL != url || check.CacheSHA256 != fmt.Sprintf("%x", sha256.Sum256(cached)) || check.CheckedAt <= 0 ||
		(check.CheckResult != "updated" && check.CheckResult != "unchanged") {
		return templateCheck{}, errors.New("template check metadata does not match the cache")
	}
	return check, nil
}

// ReadTemplate returns the cached template for rendering.
func ReadTemplate() (string, error) {
	d, err := selectedTemplateDefinition()
	if err != nil {
		return "", err
	}
	cachePath, err := d.cachePath()
	if err != nil {
		return "", err
	}
	data, _, err := readTemplateCache(cachePath)
	if err != nil {
		return "", fmt.Errorf("read template cache: %w", err)
	}
	return string(data), nil
}

func readTemplateCache(path string) ([]byte, os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, templateMaxBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > templateMaxBytes {
		return nil, nil, fmt.Errorf("template exceeds %d bytes", templateMaxBytes)
	}
	if err := validateTemplate(string(data)); err != nil {
		return nil, nil, err
	}
	return data, info, nil
}

// FetchTemplate refreshes the persisted version, shared with the background job.
func FetchTemplate() (bool, error) {
	result, err := RefreshTemplate()
	return result.Changed, err
}

// RefreshTemplate returns the status of the exact version refreshed under the
// save/refresh lock, avoiding a second settings lookup after releasing it.
func RefreshTemplate() (TemplateRefreshResult, error) {
	templateState.Lock()
	defer templateState.Unlock()
	variant, err := selectedTemplateVariant()
	if err != nil {
		return TemplateRefreshResult{}, err
	}
	d, err := resolveTemplate(variant)
	if err != nil {
		return TemplateRefreshResult{}, err
	}
	changed, err := fetchTemplateDefinitionLocked(d)
	return TemplateRefreshResult{TemplateStatus: templateStatusForDefinition(d), Changed: changed}, err
}

// Caller holds templateState; resolve the persisted catalog entry.
func fetchTemplateVariantLocked(variant string) (bool, error) {
	d, err := resolveTemplate(variant)
	if err != nil {
		return false, err
	}
	return fetchTemplateDefinitionLocked(d)
}

func fetchTemplateDefinitionLocked(d templateDefinition) (bool, error) {
	cachePath, err := d.cachePath()
	if err != nil {
		return false, err
	}
	client := &http.Client{Timeout: templateFetchTimeout}
	if d.Source == TemplateSourceRemote {
		if err := validateRemoteTemplateURL(d.URL); err != nil {
			return false, err
		}
		client = remoteTemplateClient()
	}
	return fetchTemplateWithClientLocked(d.URL, cachePath, client)
}

func fetchTemplateStateLocked(url, cachePath string) (bool, error) {
	return fetchTemplateWithClientLocked(url, cachePath, &http.Client{Timeout: templateFetchTimeout})
}

func fetchTemplateWithClientLocked(url, cachePath string, client *http.Client) (bool, error) {
	etag := templateState.etags[cachePath]
	// Missing or corrupt caches must force a full download rather than a 304.
	if _, _, err := readTemplateCache(cachePath); err != nil {
		etag = ""
	}
	cached, changed, etag, err := downloadTemplate(context.Background(), client, url, cachePath, etag)
	if err != nil {
		return false, err
	}
	result := "unchanged"
	if changed {
		result = "updated"
	}
	metadata, err := json.Marshal(templateCheck{
		URL: url, CacheSHA256: fmt.Sprintf("%x", sha256.Sum256(cached)),
		CheckedAt: time.Now().UnixMilli(), CheckResult: result,
	})
	if err != nil {
		return false, fmt.Errorf("encode template check: %w", err)
	}
	if err := publishTemplateCheck(cachePath, cached, metadata, changed); err != nil {
		return false, err
	}
	templateState.etags[cachePath] = etag
	return changed, nil
}

// fetchTemplate downloads and atomically replaces the YAML without recording a
// check. The production path stages both files through publishTemplateCheck.
func fetchTemplate(ctx context.Context, client *http.Client, url, cachePath, etag string) (bool, string, error) {
	body, changed, nextETag, err := downloadTemplate(ctx, client, url, cachePath, etag)
	if err != nil {
		return false, etag, err
	}
	if changed {
		if err := atomicWriteFile(cachePath, body); err != nil {
			return false, etag, fmt.Errorf("write template cache: %w", err)
		}
	}
	return changed, nextETag, nil
}

// downloadTemplate validates the response without publishing downloaded bytes.
func downloadTemplate(ctx context.Context, client *http.Client, url, cachePath, etag string) ([]byte, bool, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, etag, fmt.Errorf("build template request: %w", err)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, etag, fmt.Errorf("download template: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		if etag == "" {
			return nil, false, etag, errors.New("unexpected 304 without a conditional request")
		}
		cached, _, err := readTemplateCache(cachePath)
		if err != nil {
			return nil, false, etag, fmt.Errorf("304 without a valid template cache: %w", err)
		}
		return cached, false, etag, nil
	case http.StatusOK:
	default:
		return nil, false, etag, fmt.Errorf("download template: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, templateMaxBytes+1))
	if err != nil {
		return nil, false, etag, fmt.Errorf("read template body: %w", err)
	}
	if len(body) > templateMaxBytes {
		return nil, false, etag, fmt.Errorf("template exceeds %d bytes", templateMaxBytes)
	}
	if err := validateTemplate(string(body)); err != nil {
		return nil, false, etag, fmt.Errorf("template rejected: %w", err)
	}
	// ETags may be unavailable (e.g. after restarting). A 200 containing the
	// same bytes is still an unchanged template and must preserve its mtime.
	if cached, _, err := readTemplateCache(cachePath); err == nil && bytes.Equal(cached, body) {
		return body, false, resp.Header.Get("ETag"), nil
	}
	return body, true, resp.Header.Get("ETag"), nil
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
	providers, _ := parsed["proxy-providers"].(map[string]any)
	for _, value := range providers {
		provider, _ := value.(map[string]any)
		url, _ := provider["url"].(string)
		if strings.Contains(url, placeholderApiDomain) && strings.Contains(url, placeholderToken) {
			return nil
		}
	}
	return errors.New("template has no provider URL with both placeholders")
}

// publishTemplateCheck stages both files before replacing either. Publish the
// check first so a failed metadata write never exposes new subscription bytes.
// If publishing the YAML fails, restore the previous metadata as well.
func publishTemplateCheck(cachePath string, body, metadata []byte, changed bool) error {
	if !changed {
		if err := atomicWriteFile(cachePath+".check.json", metadata); err != nil {
			return fmt.Errorf("record template check: %w", err)
		}
		return nil
	}
	cacheTemp, err := stageFile(cachePath, body)
	if err != nil {
		return fmt.Errorf("write template cache: %w", err)
	}
	defer os.Remove(cacheTemp)
	checkPath := cachePath + ".check.json"
	checkTemp, err := stageFile(checkPath, metadata)
	if err != nil {
		return fmt.Errorf("record template check: %w", err)
	}
	defer os.Remove(checkTemp)
	// A hard link preserves the old record (including its mtime) for rollback;
	// using a sibling temp file keeps all renames on the same filesystem.
	backup := checkTemp + ".previous"
	if err := os.Link(checkPath, backup); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("back up template check: %w", err)
	}
	defer os.Remove(backup)
	if err := os.Rename(checkTemp, checkPath); err != nil {
		return fmt.Errorf("record template check: %w", err)
	}
	if err := os.Rename(cacheTemp, cachePath); err != nil {
		restoreErr := os.Rename(backup, checkPath)
		if os.IsNotExist(restoreErr) {
			restoreErr = os.Remove(checkPath)
		}
		return errors.Join(fmt.Errorf("write template cache: %w", err), restoreErr)
	}
	return nil
}

// atomicWriteFile writes via a sibling temp file + rename so concurrent
// readers of the cache never see a truncated template.
func atomicWriteFile(path string, data []byte) error {
	temp, err := stageFile(path, data)
	if err != nil {
		return err
	}
	defer os.Remove(temp)
	return os.Rename(temp, path)
}

func stageFile(path string, data []byte) (name string, err error) {
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
		if err != nil {
			_ = os.Remove(file.Name())
		}
	}()
	if err := file.Chmod(0o644); err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
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
