package service

import (
	xmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xservice "github.com/mhsanaei/3x-ui/v3/internal/web/service"

	submodel "github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

// ProxySource 将 3X-UI 入站和其中一个客户端配对。
// 每个 ProxySource 对应一个 Mihomo 节点。
type ProxySource struct {
	Inbound *xmodel.Inbound
	Client  *xmodel.Client
	Ref     submodel.SubscriptionInbound
}

// InboundResolver 将订阅中的入站关联展开为可导出的 (Inbound, Client) 组合。
//
// 禁用、缺失、不支持的入站以及禁用客户端会被静默跳过，避免单个旧记录
// 让整个 /feed/:token 响应失败。
type InboundResolver struct {
	inboundSvc *xservice.InboundService
}

// NewInboundResolver 返回使用 3X-UI 主库 InboundService 的 resolver。
func NewInboundResolver() *InboundResolver {
	return &InboundResolver{inboundSvc: &xservice.InboundService{}}
}

// Resolve 从主库加载完整入站，并按订阅配置选择可导出的启用客户端。
// 返回顺序遵循输入项的 SortOrder。
func (r *InboundResolver) Resolve(items []submodel.SubscriptionInbound) ([]ProxySource, error) {
	cache := make(map[int]*xmodel.Inbound)

	var sources []ProxySource
	for _, ref := range items {
		inbound, ok := cache[ref.InboundId]
		if !ok {
			loaded, err := r.inboundSvc.GetInbound(ref.InboundId)
			if err != nil || loaded == nil {
				continue
			}
			inbound = loaded
			cache[ref.InboundId] = inbound
		}
		if !IsVlessSupportedInbound(inbound, proxyOptionsFromRef(ref)) {
			continue
		}

		clients, err := r.inboundSvc.GetClients(inbound)
		if err != nil || len(clients) == 0 {
			continue
		}

		if ref.ClientEmail == "" {
			for i := range clients {
				if isExportableClient(clients[i]) {
					sources = append(sources, ProxySource{Inbound: inbound, Client: &clients[i], Ref: ref})
				}
			}
			continue
		}

		for i := range clients {
			if clients[i].Email == ref.ClientEmail && isExportableClient(clients[i]) {
				sources = append(sources, ProxySource{Inbound: inbound, Client: &clients[i], Ref: ref})
				break
			}
		}
	}

	return sources, nil
}

func isExportableClient(client xmodel.Client) bool {
	return client.Enable && client.ID != ""
}

func ProxyOptionsFromSource(source ProxySource) ProxyOptions {
	return proxyOptionsFromRef(source.Ref)
}

func proxyOptionsFromRef(ref submodel.SubscriptionInbound) ProxyOptions {
	if !ref.CdnTLS {
		return ProxyOptions{}
	}
	return ProxyOptions{CDNTLS: &CDNTLSOptions{
		Enabled:    ref.CdnTLS,
		Server:     ref.CdnServer,
		Port:       ref.CdnPort,
		Servername: ref.CdnServerName,
		ClientFp:   ref.CdnClientFp,
	}}
}
