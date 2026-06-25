package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// MihomoProxy 是 Mihomo YAML 中的一条代理节点。
//
// 字段标签面向 goccy/go-yaml 编码器。subconverter 只支持 VLESS
// TCP/xHTTP Reality、xHTTP TLS、xHTTP 明文和 xHTTP CDN TLS 的明确形态，其他传输和安全模式会在转换前被过滤。
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
	RealityOpts       *MihomoRealityOpts `yaml:"reality-opts,omitempty"`
	XHTTPOpts         *MihomoXHTTPOpts   `yaml:"xhttp-opts,omitempty"`
}

type MihomoRealityOpts struct {
	PublicKey string           `yaml:"public-key,omitempty"`
	ShortId   QuotedYAMLString `yaml:"short-id,omitempty"`
}

// QuotedYAMLString forces scalar YAML output to keep string-like numeric values quoted.
type QuotedYAMLString string

func (s QuotedYAMLString) MarshalYAML() ([]byte, error) {
	return []byte(strconv.Quote(string(s))), nil
}

type MihomoXHTTPOpts struct {
	Path                string            `yaml:"path,omitempty"`
	Host                string            `yaml:"host,omitempty"`
	Mode                string            `yaml:"mode,omitempty"`
	Headers             map[string]string `yaml:"headers,omitempty"`
	NoGRPCHeader        bool              `yaml:"no-grpc-header,omitempty"`
	XPaddingBytes       string            `yaml:"x-padding-bytes,omitempty"`
	XPaddingObfsMode    bool              `yaml:"x-padding-obfs-mode,omitempty"`
	XPaddingKey         string            `yaml:"x-padding-key,omitempty"`
	XPaddingHeader      string            `yaml:"x-padding-header,omitempty"`
	XPaddingPlacement   string            `yaml:"x-padding-placement,omitempty"`
	XPaddingMethod      string            `yaml:"x-padding-method,omitempty"`
	UplinkHTTPMethod    string            `yaml:"uplink-http-method,omitempty"`
	SessionPlacement    string            `yaml:"session-placement,omitempty"`
	SessionKey          string            `yaml:"session-key,omitempty"`
	SeqPlacement        string            `yaml:"seq-placement,omitempty"`
	SeqKey              string            `yaml:"seq-key,omitempty"`
	UplinkDataPlacement string            `yaml:"uplink-data-placement,omitempty"`
	UplinkDataKey       string            `yaml:"uplink-data-key,omitempty"`
	UplinkChunkSize     string            `yaml:"uplink-chunk-size,omitempty"`
	ScMaxEachPostBytes  string            `yaml:"sc-max-each-post-bytes,omitempty"`
	ScMinPostsInterval  string            `yaml:"sc-min-posts-interval-ms,omitempty"`
}

// ErrUnsupportedProtocol 表示入站不是 VLESS。
var ErrUnsupportedProtocol = errors.New("only VLESS protocol is supported")

// ErrUnsupportedInbound 表示入站不是本转换器支持的 VLESS 形态。
var ErrUnsupportedInbound = errors.New("only VLESS TCP/xHTTP Reality, xHTTP TLS or xHTTP inbounds are supported")

type realityInfo struct {
	Servername        string
	ShortID           string
	PublicKey         string
	ClientFingerprint string
}

type tlsInfo struct {
	Servername        string
	ClientFingerprint string
	ALPN              []string
}

type transportInfo struct {
	Network   string
	Security  string
	XHTTPOpts *MihomoXHTTPOpts
}

type ProxyOptions struct {
	CDNTLS *CDNTLSOptions
}

type CDNTLSOptions struct {
	Enabled    bool
	Server     string
	Port       int
	Servername string
	ClientFp   string
}

// IsVlessSupportedInbound 判断入站是否可以被 subconverter 导出。
// resolver 和转换器共用这条规则，避免旧订阅记录绕过前端过滤。
func IsVlessSupportedInbound(inbound *model.Inbound, opts ProxyOptions) bool {
	_, _, _, err := parseVlessSupported(inbound, opts)
	return err == nil
}

// ConvertInboundToProxy 将一个 (inbound, client) 组合转换成 Mihomo 节点。
//
//   - 当 inbound.Listen 为空或通配地址时，hostFallback 作为对外地址。
func ConvertInboundToProxy(inbound *model.Inbound, client *model.Client, hostFallback string, opts ProxyOptions) (*MihomoProxy, error) {
	if inbound == nil || client == nil {
		return nil, errors.New("inbound and client must be non-nil")
	}
	transport, reality, tls, err := parseVlessSupported(inbound, opts)
	if err != nil {
		return nil, err
	}

	proxy := &MihomoProxy{
		Name:              buildProxyName(inbound, client),
		Type:              "vless",
		Server:            resolveServerAddress(inbound.Listen, hostFallback),
		Port:              inbound.Port,
		UUID:              client.ID,
		TLS:               transport.Security == "reality" || transport.Security == "tls",
		Servername:        tls.Servername,
		ClientFingerprint: tls.ClientFingerprint,
		ALPN:              tls.ALPN,
	}
	if reality.PublicKey != "" {
		proxy.RealityOpts = &MihomoRealityOpts{
			PublicKey: reality.PublicKey,
			ShortId:   QuotedYAMLString(reality.ShortID),
		}
	}
	if transport.Network != "tcp" {
		proxy.Network = transport.Network
		proxy.XHTTPOpts = transport.XHTTPOpts
	}
	if cdn := normalizeCDNTLSOptions(opts.CDNTLS); cdn.Enabled && canApplyCDNTLSOverlay(transport, reality) {
		proxy.Server = cdn.Server
		proxy.Port = cdn.Port
		proxy.TLS = true
		proxy.Servername = cdn.Servername
		proxy.ClientFingerprint = cdn.ClientFp
		proxy.ALPN = []string{"h2"}
		if proxy.XHTTPOpts == nil {
			proxy.XHTTPOpts = &MihomoXHTTPOpts{}
		}
	}

	if proxy.TLS && proxy.ClientFingerprint == "" {
		proxy.ClientFingerprint = "chrome"
	}
	if client.Flow != "" && transport.Network == "tcp" {
		proxy.Flow = client.Flow
	}

	return proxy, nil
}

func canApplyCDNTLSOverlay(transport transportInfo, reality realityInfo) bool {
	return transport.Network == "xhttp" && reality.PublicKey == ""
}

func parseVlessSupported(inbound *model.Inbound, opts ProxyOptions) (transportInfo, realityInfo, tlsInfo, error) {
	if inbound == nil {
		return transportInfo{}, realityInfo{}, tlsInfo{}, errors.New("inbound must be non-nil")
	}
	if inbound.Protocol != model.VLESS {
		return transportInfo{}, realityInfo{}, tlsInfo{}, ErrUnsupportedProtocol
	}
	if !inbound.Enable {
		return transportInfo{}, realityInfo{}, tlsInfo{}, ErrUnsupportedInbound
	}

	stream := map[string]any{}
	if inbound.StreamSettings != "" {
		if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
			return transportInfo{}, realityInfo{}, tlsInfo{}, fmt.Errorf("parse streamSettings: %w", err)
		}
	}
	network, _ := stream["network"].(string)
	security, _ := stream["security"].(string)
	if network == "" {
		network = "tcp"
	}
	if hasExternalProxy(stream) {
		return transportInfo{}, realityInfo{}, tlsInfo{}, ErrUnsupportedInbound
	}

	transport := transportInfo{Network: network, Security: security}
	switch network {
	case "tcp":
		if !hasSupportedTCPSettings(stream) {
			return transportInfo{}, realityInfo{}, tlsInfo{}, ErrUnsupportedInbound
		}
	case "xhttp":
		transport.XHTTPOpts = parseXHTTPOpts(stream)
	default:
		return transportInfo{}, realityInfo{}, tlsInfo{}, ErrUnsupportedInbound
	}

	switch security {
	case "reality":
		reality, tls, err := parseRealityInfo(stream)
		return transport, reality, tls, err
	case "tls":
		if network == "xhttp" {
			return transport, realityInfo{}, parseTLSInfo(stream), nil
		}
		return transportInfo{}, realityInfo{}, tlsInfo{}, ErrUnsupportedInbound
	case "none", "":
		if network == "xhttp" {
			return transport, realityInfo{}, tlsInfo{}, nil
		}
		return transportInfo{}, realityInfo{}, tlsInfo{}, ErrUnsupportedInbound
	default:
		return transportInfo{}, realityInfo{}, tlsInfo{}, ErrUnsupportedInbound
	}
}

func parseTLSInfo(stream map[string]any) tlsInfo {
	tlsSetting, _ := stream["tlsSettings"].(map[string]any)
	if tlsSetting == nil {
		return tlsInfo{}
	}

	info := tlsInfo{}
	if servername, ok := tlsSetting["serverName"].(string); ok {
		info.Servername = strings.TrimSpace(servername)
	}
	if fp, ok := tlsSetting["fingerprint"].(string); ok {
		info.ClientFingerprint = strings.TrimSpace(fp)
	}
	if settings, ok := tlsSetting["settings"].(map[string]any); ok {
		if fp, ok := settings["fingerprint"].(string); ok {
			info.ClientFingerprint = strings.TrimSpace(fp)
		}
	}
	info.ALPN = stringList(tlsSetting["alpn"])
	return info
}

func parseRealityInfo(stream map[string]any) (realityInfo, tlsInfo, error) {
	realitySetting, _ := stream["realitySettings"].(map[string]any)
	if realitySetting == nil {
		return realityInfo{}, tlsInfo{}, ErrUnsupportedInbound
	}

	info := realityInfo{}
	if sNames, ok := realitySetting["serverNames"].([]any); ok && len(sNames) > 0 {
		if sn, ok := sNames[0].(string); ok {
			info.Servername = sn
		}
	}
	if shortIDs, ok := realitySetting["shortIds"].([]any); ok && len(shortIDs) > 0 {
		if sid, ok := shortIDs[0].(string); ok {
			info.ShortID = sid
		}
	}
	if settings, ok := realitySetting["settings"].(map[string]any); ok {
		if pbk, ok := settings["publicKey"].(string); ok {
			info.PublicKey = pbk
		}
		if fp, ok := settings["fingerprint"].(string); ok {
			info.ClientFingerprint = fp
		}
	}
	if info.Servername == "" || info.PublicKey == "" {
		return realityInfo{}, tlsInfo{}, ErrUnsupportedInbound
	}

	return info, tlsInfo{
		Servername:        info.Servername,
		ClientFingerprint: info.ClientFingerprint,
	}, nil
}

func stringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	return out
}

func normalizeCDNTLSOptions(in *CDNTLSOptions) CDNTLSOptions {
	if in == nil || !in.Enabled {
		return CDNTLSOptions{}
	}
	out := *in
	out.Server = strings.TrimSpace(out.Server)
	out.Servername = strings.TrimSpace(out.Servername)
	out.ClientFp = strings.TrimSpace(out.ClientFp)
	if out.Port == 0 {
		out.Port = 443
	}
	if out.Servername == "" {
		out.Servername = out.Server
	}
	if out.ClientFp == "" {
		out.ClientFp = "chrome"
	}
	if out.Server == "" {
		return CDNTLSOptions{}
	}
	out.Enabled = true
	return out
}

func parseXHTTPOpts(stream map[string]any) *MihomoXHTTPOpts {
	xhttp, _ := stream["xhttpSettings"].(map[string]any)
	if xhttp == nil {
		return nil
	}

	opts := &MihomoXHTTPOpts{}
	if path, ok := xhttp["path"].(string); ok && path != "" {
		opts.Path = path
	}
	if host, ok := xhttp["host"].(string); ok && host != "" {
		opts.Host = host
	} else if headers, ok := xhttp["headers"].(map[string]any); ok {
		opts.Host = searchHeaderHost(headers)
	}
	if mode, ok := xhttp["mode"].(string); ok && mode != "" {
		opts.Mode = mode
	}

	applyXHTTPString(xhttp, "xPaddingBytes", &opts.XPaddingBytes)
	if obfs, ok := xhttp["xPaddingObfsMode"].(bool); ok && obfs {
		opts.XPaddingObfsMode = true
		applyXHTTPString(xhttp, "xPaddingKey", &opts.XPaddingKey)
		applyXHTTPString(xhttp, "xPaddingHeader", &opts.XPaddingHeader)
		applyXHTTPString(xhttp, "xPaddingPlacement", &opts.XPaddingPlacement)
		applyXHTTPString(xhttp, "xPaddingMethod", &opts.XPaddingMethod)
	}
	applyXHTTPString(xhttp, "uplinkHTTPMethod", &opts.UplinkHTTPMethod)
	applyXHTTPString(xhttp, "sessionPlacement", &opts.SessionPlacement)
	applyXHTTPString(xhttp, "sessionKey", &opts.SessionKey)
	applyXHTTPString(xhttp, "seqPlacement", &opts.SeqPlacement)
	applyXHTTPString(xhttp, "seqKey", &opts.SeqKey)
	applyXHTTPString(xhttp, "uplinkDataPlacement", &opts.UplinkDataPlacement)
	applyXHTTPString(xhttp, "uplinkDataKey", &opts.UplinkDataKey)
	applyXHTTPStringExceptDefault(xhttp, "scMaxEachPostBytes", "1000000", &opts.ScMaxEachPostBytes)
	applyXHTTPStringExceptDefault(xhttp, "scMinPostsIntervalMs", "30", &opts.ScMinPostsInterval)
	if chunkSize := xhttpNonZeroString(xhttp["uplinkChunkSize"]); chunkSize != "" {
		opts.UplinkChunkSize = chunkSize
	}
	if noGRPCHeader, ok := xhttp["noGRPCHeader"].(bool); ok && noGRPCHeader {
		opts.NoGRPCHeader = true
	}
	if headers, ok := xhttp["headers"].(map[string]any); ok {
		opts.Headers = xhttpHeaders(headers)
	}

	if !hasXHTTPOpts(opts) {
		return nil
	}
	return opts
}

func applyXHTTPString(xhttp map[string]any, field string, out *string) {
	if value, ok := xhttp[field].(string); ok && value != "" {
		*out = value
	}
}

func applyXHTTPStringExceptDefault(xhttp map[string]any, field, defaultValue string, out *string) {
	if value := xhttpNonZeroString(xhttp[field]); value != "" && value != defaultValue {
		*out = value
	}
}

func xhttpNonZeroString(value any) string {
	switch typed := value.(type) {
	case string:
		v := strings.TrimSpace(typed)
		if v != "" && v != "0" {
			return v
		}
	case int:
		if typed != 0 {
			return fmt.Sprint(typed)
		}
	case int32:
		if typed != 0 {
			return fmt.Sprint(typed)
		}
	case int64:
		if typed != 0 {
			return fmt.Sprint(typed)
		}
	case float32:
		if typed != 0 {
			return strconv.FormatFloat(float64(typed), 'f', -1, 32)
		}
	case float64:
		if typed != 0 {
			return strconv.FormatFloat(typed, 'f', -1, 64)
		}
	}
	return ""
}

func xhttpHeaders(headers map[string]any) map[string]string {
	out := map[string]string{}
	for key, value := range headers {
		if strings.EqualFold(key, "host") {
			continue
		}
		if header := xhttpHeaderValue(value); header != "" {
			out[key] = header
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func xhttpHeaderValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		if len(typed) == 0 {
			return ""
		}
		header, _ := typed[0].(string)
		return header
	default:
		return ""
	}
}

func hasXHTTPOpts(opts *MihomoXHTTPOpts) bool {
	return opts.Path != "" ||
		opts.Host != "" ||
		opts.Mode != "" ||
		len(opts.Headers) > 0 ||
		opts.NoGRPCHeader ||
		opts.XPaddingBytes != "" ||
		opts.XPaddingObfsMode ||
		opts.XPaddingKey != "" ||
		opts.XPaddingHeader != "" ||
		opts.XPaddingPlacement != "" ||
		opts.XPaddingMethod != "" ||
		opts.UplinkHTTPMethod != "" ||
		opts.SessionPlacement != "" ||
		opts.SessionKey != "" ||
		opts.SeqPlacement != "" ||
		opts.SeqKey != "" ||
		opts.UplinkDataPlacement != "" ||
		opts.UplinkDataKey != "" ||
		opts.UplinkChunkSize != "" ||
		opts.ScMaxEachPostBytes != "" ||
		opts.ScMinPostsInterval != ""
}

func searchHeaderHost(headers map[string]any) string {
	for key, value := range headers {
		if !strings.EqualFold(key, "host") {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed
		case []any:
			if len(typed) == 0 {
				return ""
			}
			host, _ := typed[0].(string)
			return host
		}
	}
	return ""
}

func hasSupportedTCPSettings(stream map[string]any) bool {
	tcp, _ := stream["tcpSettings"].(map[string]any)
	if tcp == nil {
		return true
	}
	header, _ := tcp["header"].(map[string]any)
	if header == nil {
		return true
	}
	typeStr, _ := header["type"].(string)
	return typeStr == "" || typeStr == "none"
}

func hasExternalProxy(stream map[string]any) bool {
	v, ok := stream["externalProxy"]
	if !ok || v == nil {
		return false
	}
	items, ok := v.([]any)
	return !ok || len(items) > 0
}

// resolveServerAddress 对齐 3X-UI 分享链接的地址兜底规则：
// 非通配 Listen 优先，否则使用当前订阅请求的 host。
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
