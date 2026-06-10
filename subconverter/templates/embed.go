// Package templates exposes the bundled Mihomo YAML template through embed.FS
// so the binary stays self-contained and the template ships with the build.
package templates

import _ "embed"

// MihomoTemplate is the full Mihomo subscription template, with the placeholders
// __TOKEN__ and __API_DOMAIN__ resolved by the renderer.
//
//go:embed mihomo.yaml
var MihomoTemplate string
