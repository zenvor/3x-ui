package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// MihomoProxy 是 Mihomo YAML 中的一条代理节点。
//
// 字段标签面向 goccy/go-yaml 编码器。subconverter 只支持普通
// VLESS TCP Reality 入站，其他传输和安全模式会在转换前被过滤。
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
}

type MihomoRealityOpts struct {
	PublicKey string `yaml:"public-key,omitempty"`
	ShortId   string `yaml:"short-id,omitempty"`
}

// ErrUnsupportedProtocol 表示入站不是 VLESS。
var ErrUnsupportedProtocol = errors.New("only VLESS protocol is supported")

// ErrUnsupportedInbound 表示入站不是本转换器支持的 VLESS TCP Reality 形态。
var ErrUnsupportedInbound = errors.New("only VLESS TCP Reality inbounds are supported")

type realityInfo struct {
	Servername        string
	ShortID           string
	PublicKey         string
	ClientFingerprint string
}

// IsVlessTCPRealityInbound 判断入站是否可以被 subconverter 导出。
// resolver 和转换器共用这条规则，避免旧订阅记录绕过前端过滤。
func IsVlessTCPRealityInbound(inbound *model.Inbound) bool {
	_, _, err := parseVlessTCPReality(inbound)
	return err == nil
}

// ConvertInboundToProxy 将一个 (inbound, client) 组合转换成 Mihomo 节点。
//
//   - 当 inbound.Listen 为空或通配地址时，hostFallback 作为对外地址。
func ConvertInboundToProxy(inbound *model.Inbound, client *model.Client, hostFallback string) (*MihomoProxy, error) {
	if inbound == nil || client == nil {
		return nil, errors.New("inbound and client must be non-nil")
	}
	_, reality, err := parseVlessTCPReality(inbound)
	if err != nil {
		return nil, err
	}

	proxy := &MihomoProxy{
		Name:              buildProxyName(inbound, client),
		Type:              "vless",
		Server:            resolveServerAddress(inbound.Listen, hostFallback),
		Port:              inbound.Port,
		UUID:              client.ID,
		TLS:               true,
		Servername:        reality.Servername,
		ClientFingerprint: reality.ClientFingerprint,
		RealityOpts: &MihomoRealityOpts{
			PublicKey: reality.PublicKey,
			ShortId:   reality.ShortID,
		},
	}

	if proxy.ClientFingerprint == "" {
		proxy.ClientFingerprint = "chrome"
	}
	if client.Flow != "" {
		proxy.Flow = client.Flow
	}

	return proxy, nil
}

func parseVlessTCPReality(inbound *model.Inbound) (map[string]any, realityInfo, error) {
	if inbound == nil {
		return nil, realityInfo{}, errors.New("inbound must be non-nil")
	}
	if inbound.Protocol != model.VLESS {
		return nil, realityInfo{}, ErrUnsupportedProtocol
	}
	if !inbound.Enable {
		return nil, realityInfo{}, ErrUnsupportedInbound
	}

	stream := map[string]any{}
	if inbound.StreamSettings != "" {
		if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
			return nil, realityInfo{}, fmt.Errorf("parse streamSettings: %w", err)
		}
	}
	network, _ := stream["network"].(string)
	security, _ := stream["security"].(string)
	if network != "tcp" || security != "reality" {
		return nil, realityInfo{}, ErrUnsupportedInbound
	}
	if !hasSupportedTCPSettings(stream) || hasExternalProxy(stream) {
		return nil, realityInfo{}, ErrUnsupportedInbound
	}

	realitySetting, _ := stream["realitySettings"].(map[string]any)
	if realitySetting == nil {
		return nil, realityInfo{}, ErrUnsupportedInbound
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
		return nil, realityInfo{}, ErrUnsupportedInbound
	}

	return stream, info, nil
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
