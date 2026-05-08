package service

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestRenderMihomoYAMLSubstitutesPlaceholders(t *testing.T) {
	out := RenderMihomoYAML("https://panel.example.com", "abc123")

	if strings.Contains(out, "__TOKEN__") || strings.Contains(out, "__API_DOMAIN__") {
		t.Fatal("template still contains unresolved placeholders")
	}
	if !strings.Contains(out, "https://panel.example.com/feed/abc123/nodes") {
		t.Fatalf("expected provider URL with substituted token+domain, got:\n%s", out)
	}
}

func TestRenderMihomoYAMLIsValidYAML(t *testing.T) {
	out := RenderMihomoYAML("https://x", "tok")

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("rendered template is not valid YAML: %v", err)
	}
	if _, ok := parsed["proxy-providers"]; !ok {
		t.Error("rendered template missing proxy-providers")
	}
	if _, ok := parsed["rules"]; !ok {
		t.Error("rendered template missing rules")
	}
}

func TestRenderProviderYAMLEmptyList(t *testing.T) {
	out, err := RenderProviderYAML(nil)
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

func TestRenderProviderYAMLRoundTrip(t *testing.T) {
	in := []MihomoProxy{
		{
			Name:   "node-1",
			Type:   "vless",
			Server: "1.2.3.4",
			Port:   443,
			UUID:   "uuid-a",
			TLS:    true,
			ALPN:   []string{"h2"},
			WSOpts: &MihomoWSOpts{Path: "/ws"},
		},
	}
	out, err := RenderProviderYAML(in)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "name: node-1") {
		t.Errorf("expected proxy name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "ws-opts:") {
		t.Errorf("expected ws-opts block, got:\n%s", out)
	}
	if !strings.Contains(out, "path: /ws") {
		t.Errorf("expected ws-opts.path, got:\n%s", out)
	}
}
