package service

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v2/subconverter/templates"
)

const (
	placeholderToken     = "__TOKEN__"
	placeholderApiDomain = "__API_DOMAIN__"
)

// RenderMihomoYAML returns the full Mihomo subscription YAML with placeholders
// resolved.
//
// apiDomain should be a scheme+host string such as "https://panel.example.com",
// derived from the public request (X-Forwarded-{Proto,Host} when behind a
// reverse proxy, otherwise the direct Host header).
func RenderMihomoYAML(apiDomain, token string) string {
	out := templates.MihomoTemplate
	out = strings.ReplaceAll(out, placeholderApiDomain, apiDomain)
	out = strings.ReplaceAll(out, placeholderToken, token)
	return out
}

// RenderProviderYAML returns a Mihomo proxy-provider document (a single
// "proxies:" map) holding the given proxies.
//
// An empty list still produces "proxies: []" so Mihomo clients see a valid
// document instead of a parsing error.
func RenderProviderYAML(proxies []MihomoProxy) (string, error) {
	if proxies == nil {
		proxies = []MihomoProxy{}
	}
	out, err := yaml.Marshal(map[string]any{"proxies": proxies})
	if err != nil {
		return "", fmt.Errorf("marshal provider yaml: %w", err)
	}
	return string(out), nil
}
