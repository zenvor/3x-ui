package service

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	placeholderToken     = "__TOKEN__"
	placeholderApiDomain = "__API_DOMAIN__"
)

// RenderMihomoYAML substitutes placeholders in the cached template (see template.go),
// erroring when nothing is cached so the caller answers 503. apiDomain is the public scheme+host.
func RenderMihomoYAML(apiDomain, token string) (string, error) {
	tpl, err := ReadTemplate()
	if err != nil {
		return "", err
	}
	out := strings.ReplaceAll(tpl, placeholderApiDomain, apiDomain)
	out = strings.ReplaceAll(out, placeholderToken, token)
	return out, nil
}

// RenderMihomoProviderYAML returns a Mihomo proxy-provider document (a single
// "proxies:" map) holding the given proxies.
//
// An empty list still produces "proxies: []" so Mihomo clients see a valid
// document instead of a parsing error.
func RenderMihomoProviderYAML(proxies []MihomoProxy) (string, error) {
	if proxies == nil {
		proxies = []MihomoProxy{}
	}
	out, err := yaml.Marshal(map[string]any{"proxies": proxies})
	if err != nil {
		return "", fmt.Errorf("marshal provider yaml: %w", err)
	}
	return string(out), nil
}
