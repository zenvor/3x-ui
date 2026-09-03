package service

import (
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// The template's content now lives in the mihomo-config repository; tests here
// only cover the renderer's contract (placeholder substitution, cache miss).
const testCachedTemplate = `mixed-port: 7890
proxy-providers:
  main:
    type: http
    url: '__API_DOMAIN__/feed/__TOKEN__/nodes'
rules:
  - MATCH,PROXY
`

func seedTemplateCache(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(TemplateCachePath(), []byte(testCachedTemplate), 0o644); err != nil {
		t.Fatalf("seed template cache: %v", err)
	}
}

func TestRenderMihomoYAMLSubstitutesPlaceholders(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	seedTemplateCache(t)

	out, err := RenderMihomoYAML("https://panel.example.com", "abc123")
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if strings.Contains(out, "__TOKEN__") || strings.Contains(out, "__API_DOMAIN__") {
		t.Fatal("template still contains unresolved placeholders")
	}
	if !strings.Contains(out, "https://panel.example.com/feed/abc123/nodes") {
		t.Fatalf("expected provider URL with substituted token+domain, got:\n%s", out)
	}
}

func TestRenderMihomoYAMLWithoutCacheFails(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())

	if _, err := RenderMihomoYAML("https://x", "tok"); err == nil {
		t.Fatal("expected error when no template is cached")
	}
}

func TestRenderMihomoProviderYAMLEmptyList(t *testing.T) {
	out, err := RenderMihomoProviderYAML(nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "proxies:") {
		t.Fatalf("output should contain 'proxies:' even when empty, got:\n%s", out)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}
	if proxies, _ := parsed["proxies"].([]any); len(proxies) != 0 {
		t.Fatalf("proxies should be empty list, got %v", proxies)
	}
}

func TestRenderMihomoProviderYAMLRoundTrip(t *testing.T) {
	in := []MihomoProxy{
		{
			Name:   "node-1",
			Type:   "vless",
			Server: "1.2.3.4",
			Port:   443,
			UUID:   "uuid-a",
			TLS:    true,
			RealityOpts: &MihomoRealityOpts{
				PublicKey: "pubkey-a",
				ShortId:   "08",
			},
			Network: "xhttp",
			XHTTPOpts: &MihomoXHTTPOpts{
				Path:               "/abc123",
				Host:               "cdn.example.com",
				Mode:               "auto",
				XPaddingBytes:      "100-1000",
				UplinkHTTPMethod:   "PUT",
				UplinkChunkSize:    "8192",
				NoGRPCHeader:       true,
				ScMaxEachPostBytes: "2000000",
			},
		},
	}
	out, err := RenderMihomoProviderYAML(in)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "name: node-1") {
		t.Errorf("expected proxy name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "reality-opts:") {
		t.Errorf("expected reality-opts block, got:\n%s", out)
	}
	if !strings.Contains(out, "public-key: pubkey-a") {
		t.Errorf("expected reality public key, got:\n%s", out)
	}
	if !strings.Contains(out, `short-id: "08"`) {
		t.Errorf("expected quoted reality short id, got:\n%s", out)
	}
	if !strings.Contains(out, "xhttp-opts:") || !strings.Contains(out, "mode: auto") {
		t.Errorf("expected xhttp options, got:\n%s", out)
	}
	for _, want := range []string{
		"x-padding-bytes: 100-1000",
		"uplink-http-method: PUT",
		"uplink-chunk-size: \"8192\"",
		"no-grpc-header: true",
		"sc-max-each-post-bytes: \"2000000\"",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in xhttp options, got:\n%s", want, out)
		}
	}
}
