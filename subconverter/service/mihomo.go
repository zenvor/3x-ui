package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

// MihomoProxy is one proxy entry in a Mihomo YAML.
//
// Field tags target the goccy/go-yaml encoder (already in 3X-UI's go.mod).
// Only the subset of Mihomo fields the MVP needs is modelled here; rare
// transports (kcp/httpupgrade/xhttp) are deliberately omitted.
type MihomoProxy struct {
	Name              string             `yaml:"name"`
	Type              string             `yaml:"type"`
	Server            string             `yaml:"server"`
	Port              int                `yaml:"port"`
	UUID              string             `yaml:"uuid"`
	Network           string             `yaml:"network,omitempty"`
	TLS               bool               `yaml:"tls,omitempty"`
	Servername        string             `yaml:"servername,omitempty"`
	ClientFingerprint string             `yaml:"client-fingerprint,omitempty"`
	Flow              string             `yaml:"flow,omitempty"`
	ALPN              []string           `yaml:"alpn,omitempty"`
	WSOpts            *MihomoWSOpts      `yaml:"ws-opts,omitempty"`
	GRPCOpts          *MihomoGRPCOpts    `yaml:"grpc-opts,omitempty"`
	RealityOpts       *MihomoRealityOpts `yaml:"reality-opts,omitempty"`
}

type MihomoWSOpts struct {
	Path    string            `yaml:"path,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

type MihomoGRPCOpts struct {
	GrpcServiceName string `yaml:"grpc-service-name,omitempty"`
}

type MihomoRealityOpts struct {
	PublicKey string `yaml:"public-key,omitempty"`
	ShortId   string `yaml:"short-id,omitempty"`
}

// ErrUnsupportedProtocol means the inbound is not VLESS. The MVP intentionally
// only supports VLESS — other protocols are skipped at the source list layer.
var ErrUnsupportedProtocol = errors.New("only VLESS protocol is supported")

// ConvertInboundToProxy turns a single (inbound, client) pair into one Mihomo
// proxy entry.
//
//   - hostFallback is used when inbound.Listen is 0.0.0.0 / ::/empty (i.e. the
//     inbound is bound to all interfaces and the public-facing address must
//     come from the request host).
func ConvertInboundToProxy(inbound *model.Inbound, client *model.Client, hostFallback string) (*MihomoProxy, error) {
	if inbound == nil || client == nil {
		return nil, errors.New("inbound and client must be non-nil")
	}
	if inbound.Protocol != model.VLESS {
		return nil, ErrUnsupportedProtocol
	}

	stream := map[string]any{}
	if inbound.StreamSettings != "" {
		if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
			return nil, fmt.Errorf("parse streamSettings: %w", err)
		}
	}

	network, _ := stream["network"].(string)
	security, _ := stream["security"].(string)

	proxy := &MihomoProxy{
		Name:   buildProxyName(inbound, client),
		Type:   "vless",
		Server: resolveServerAddress(inbound.Listen, hostFallback),
		Port:   inbound.Port,
		UUID:   client.ID,
	}
	if network != "" && network != "tcp" {
		proxy.Network = network
	}

	applyTransport(proxy, network, stream)
	applySecurity(proxy, security, network, stream, client)

	if proxy.TLS && proxy.ClientFingerprint == "" {
		proxy.ClientFingerprint = "chrome"
	}

	return proxy, nil
}

// resolveServerAddress mirrors 3X-UI's genVlessLink fallback rule:
// use the inbound's Listen unless it's a wildcard, in which case use whatever
// host the public request was issued against.
func resolveServerAddress(listen, hostFallback string) string {
	switch listen {
	case "", "0.0.0.0", "::", "::0":
		return hostFallback
	default:
		return listen
	}
}

func buildProxyName(inbound *model.Inbound, client *model.Client) string {
	switch {
	case inbound.Remark != "" && client.Email != "":
		return inbound.Remark + "-" + client.Email
	case inbound.Remark != "":
		return inbound.Remark
	case client.Email != "":
		return client.Email
	default:
		return fmt.Sprintf("inbound-%d", inbound.Id)
	}
}

func applyTransport(proxy *MihomoProxy, network string, stream map[string]any) {
	switch network {
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]any)
		if ws == nil {
			return
		}
		opts := &MihomoWSOpts{}
		if path, ok := ws["path"].(string); ok {
			opts.Path = path
		}
		host := ""
		if h, ok := ws["host"].(string); ok && h != "" {
			host = h
		} else if headers, ok := ws["headers"].(map[string]any); ok {
			host = pickHostHeader(headers)
		}
		if host != "" {
			opts.Headers = map[string]string{"Host": host}
		}
		proxy.WSOpts = opts
	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]any)
		if grpc == nil {
			return
		}
		sn, _ := grpc["serviceName"].(string)
		proxy.GRPCOpts = &MihomoGRPCOpts{GrpcServiceName: sn}
	}
}

func applySecurity(proxy *MihomoProxy, security, network string, stream map[string]any, client *model.Client) {
	switch security {
	case "tls":
		proxy.TLS = true
		if tlsSetting, ok := stream["tlsSettings"].(map[string]any); ok && tlsSetting != nil {
			if sn, ok := tlsSetting["serverName"].(string); ok && sn != "" {
				proxy.Servername = sn
			}
			if alpns, ok := tlsSetting["alpn"].([]any); ok {
				for _, a := range alpns {
					if s, ok := a.(string); ok {
						proxy.ALPN = append(proxy.ALPN, s)
					}
				}
			}
			if fp := nestedFingerprint(tlsSetting); fp != "" {
				proxy.ClientFingerprint = fp
			}
		}
		if proxy.Servername == "" {
			proxy.Servername = proxy.Server
		}
		if network == "tcp" && client.Flow != "" {
			proxy.Flow = client.Flow
		}
	case "reality":
		proxy.TLS = true
		realitySetting, _ := stream["realitySettings"].(map[string]any)
		if realitySetting != nil {
			opts := &MihomoRealityOpts{}
			if sNames, ok := realitySetting["serverNames"].([]any); ok && len(sNames) > 0 {
				if sn, ok := sNames[0].(string); ok {
					proxy.Servername = sn
				}
			}
			if shortIds, ok := realitySetting["shortIds"].([]any); ok && len(shortIds) > 0 {
				if sid, ok := shortIds[0].(string); ok {
					opts.ShortId = sid
				}
			}
			if settings, ok := realitySetting["settings"].(map[string]any); ok {
				if pbk, ok := settings["publicKey"].(string); ok {
					opts.PublicKey = pbk
				}
				if fp, ok := settings["fingerprint"].(string); ok && fp != "" {
					proxy.ClientFingerprint = fp
				}
			}
			proxy.RealityOpts = opts
		}
		if network == "tcp" && client.Flow != "" {
			proxy.Flow = client.Flow
		}
	}
}

// pickHostHeader scans an HTTP-headers map for the Host entry. Header keys are
// case-insensitive in HTTP but the JSON object preserves original casing, so
// we have to fold both sides.
func pickHostHeader(headers map[string]any) string {
	for k, v := range headers {
		if strings.EqualFold(k, "Host") {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func nestedFingerprint(tlsSetting map[string]any) string {
	settings, ok := tlsSetting["settings"].(map[string]any)
	if !ok {
		return ""
	}
	if fp, ok := settings["fingerprint"].(string); ok {
		return fp
	}
	return ""
}
