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

func TestRenderMihomoYAMLIncludesLeakProtectionByDefault(t *testing.T) {
	out := RenderMihomoYAML("https://x", "tok")

	for _, rule := range []string{
		"allow-lan: true\nbind-address: \"*\"",
		"AND,((NETWORK,udp),(DST-PORT,3478-3481)),REJECT",
		"AND,((NETWORK,udp),(DST-PORT,5349)),REJECT",
		"AND,((NETWORK,udp),(DST-PORT,19302-19309)),REJECT",
		"udp: true",
		"ip-version: ipv4",
		"AND,((NETWORK,UDP),(DST-PORT,443)),REJECT",
		"NETWORK,udp,REJECT",
		"direct-nameserver:\n    - https://doh.pub/dns-query\n    - https://dns.alidns.com/dns-query",
		"health-check:\n      enable: false\n      url: https://www.gstatic.com/generate_204\n      interval: 0\n      timeout: 5000\n      lazy: true\n      expected-status: 204",
	} {
		if !strings.Contains(out, rule) {
			t.Fatalf("expected leak protection setting %q in output:\n%s", rule, out)
		}
	}

	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("rendered template with leak protection is not valid YAML: %v", err)
	}
	if strings.Contains(out, "ip-version: ipv6-prefer") {
		t.Fatalf("provider should use Mihomo's default dual ip-version instead of forcing ipv6-prefer:\n%s", out)
	}
	if strings.Contains(out, "fallback-filter:\n    geoip: true\n    geoip-code: CN\n    ipcidr:\n      - 240.0.0.0/4\n    geosite:") {
		t.Fatalf("fallback-filter.geosite is deprecated; nameserver-policy should own geosite routing:\n%s", out)
	}
	for _, unexpected := range []string{
		"interval: 86400",
	} {
		if strings.Contains(out, unexpected) {
			t.Fatalf("expected provider auto-update setting %q to be removed:\n%s", unexpected, out)
		}
	}
}

func TestRenderMihomoYAMLUsesStringNameserverPolicy(t *testing.T) {
	out := RenderMihomoYAML("https://x", "tok")

	var parsed struct {
		DNS struct {
			NameserverPolicy map[string]string `yaml:"nameserver-policy"`
		} `yaml:"dns"`
	}
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("nameserver-policy should be compatible with string-only Clash Meta parsers: %v", err)
	}

	want := map[string]string{
		"geosite:cn,private":          "https://doh.pub/dns-query",
		"geosite:gfw,geolocation-!cn": "https://8.8.8.8/dns-query#PROXY",
	}
	for key, value := range want {
		if parsed.DNS.NameserverPolicy[key] != value {
			t.Fatalf("nameserver-policy[%q] = %q, want %q", key, parsed.DNS.NameserverPolicy[key], value)
		}
	}
}

func TestRenderMihomoYAMLMatchesLegacyClashMetaShape(t *testing.T) {
	out := RenderMihomoYAML("https://x", "tok")

	var parsed struct {
		DNS struct {
			DefaultNameserver     []string          `yaml:"default-nameserver"`
			FakeIPFilter          []string          `yaml:"fake-ip-filter"`
			NameserverPolicy      map[string]string `yaml:"nameserver-policy"`
			Nameserver            []string          `yaml:"nameserver"`
			Fallback              []string          `yaml:"fallback"`
			ProxyServerNameserver []string          `yaml:"proxy-server-nameserver"`
			DirectNameserver      []string          `yaml:"direct-nameserver"`
			FallbackFilter        struct {
				GeoIP  bool     `yaml:"geoip"`
				IPCIDR []string `yaml:"ipcidr"`
			} `yaml:"fallback-filter"`
		} `yaml:"dns"`
		Sniffer struct {
			Sniff map[string]struct {
				Ports               []string `yaml:"ports"`
				OverrideDestination *bool    `yaml:"override-destination"`
			} `yaml:"sniff"`
			SkipDomain []string `yaml:"skip-domain"`
		} `yaml:"sniffer"`
		ProxyProviders map[string]struct {
			Type        string `yaml:"type"`
			URL         string `yaml:"url"`
			Path        string `yaml:"path"`
			HealthCheck struct {
				Enable         bool   `yaml:"enable"`
				URL            string `yaml:"url"`
				Interval       int    `yaml:"interval"`
				Timeout        int    `yaml:"timeout"`
				Lazy           bool   `yaml:"lazy"`
				ExpectedStatus int    `yaml:"expected-status"`
			} `yaml:"health-check"`
		} `yaml:"proxy-providers"`
		ProxyGroups []struct {
			Name string   `yaml:"name"`
			Type string   `yaml:"type"`
			Use  []string `yaml:"use"`
		} `yaml:"proxy-groups"`
		Rules []string `yaml:"rules"`
	}
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("rendered template should match legacy Clash Meta string/list shape: %v", err)
	}

	if len(parsed.DNS.NameserverPolicy) != 2 {
		t.Fatalf("nameserver-policy should have 2 string entries, got %#v", parsed.DNS.NameserverPolicy)
	}
	if len(parsed.Sniffer.Sniff["TLS"].Ports) == 0 || len(parsed.Sniffer.Sniff["HTTP"].Ports) == 0 {
		t.Fatalf("sniffer ports should remain parseable as string lists: %#v", parsed.Sniffer.Sniff)
	}
	if mainProvider, ok := parsed.ProxyProviders["main"]; !ok || mainProvider.Type != "http" {
		t.Fatalf("main proxy provider should remain an HTTP provider, got %#v", parsed.ProxyProviders)
	} else if mainProvider.HealthCheck.Enable ||
		mainProvider.HealthCheck.URL != "https://www.gstatic.com/generate_204" ||
		mainProvider.HealthCheck.Interval != 0 ||
		mainProvider.HealthCheck.Timeout != 5000 ||
		!mainProvider.HealthCheck.Lazy ||
		mainProvider.HealthCheck.ExpectedStatus != 204 {
		t.Fatalf("main proxy provider should keep manual-only health-check settings, got %#v", mainProvider.HealthCheck)
	}
	if len(parsed.ProxyGroups) != 1 || len(parsed.ProxyGroups[0].Use) != 1 || parsed.ProxyGroups[0].Use[0] != "main" {
		t.Fatalf("proxy group should keep provider use list, got %#v", parsed.ProxyGroups)
	}
	if len(parsed.Rules) == 0 {
		t.Fatal("rules should remain a list of strings")
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
				ShortId:   "abcd",
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
}
