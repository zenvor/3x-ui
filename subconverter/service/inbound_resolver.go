package service

import (
	xmodel "github.com/mhsanaei/3x-ui/v2/database/model"
	xservice "github.com/mhsanaei/3x-ui/v2/web/service"

	submodel "github.com/mhsanaei/3x-ui/v2/subconverter/model"
)

// ProxySource pairs a 3X-UI inbound with one of its clients. Each
// ProxySource maps 1:1 to a Mihomo proxy entry.
type ProxySource struct {
	Inbound *xmodel.Inbound
	Client  *xmodel.Client
}

// InboundResolver expands a subscription's inbound junction list into the
// concrete (Inbound, Client) pairs that will become Mihomo proxy entries.
//
// Disabled inbounds, missing inbounds, non-VLESS inbounds and disabled clients
// are dropped silently — the public /feed/:token endpoint then yields a YAML
// without those entries rather than failing the whole request. Stage 5's UI
// is responsible for filtering these cases at creation time.
type InboundResolver struct {
	inboundSvc *xservice.InboundService
}

// NewInboundResolver returns a resolver that uses 3X-UI's InboundService
// (which talks to the upstream 3x-ui.db).
func NewInboundResolver() *InboundResolver {
	return &InboundResolver{inboundSvc: &xservice.InboundService{}}
}

// Resolve loads the referenced inbounds from the upstream 3X-UI db and pairs
// them with their selected clients. Order of the returned slice follows the
// SortOrder of the input.
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
		if !inbound.Enable || inbound.Protocol != xmodel.VLESS {
			continue
		}

		clients, err := r.inboundSvc.GetClients(inbound)
		if err != nil || len(clients) == 0 {
			continue
		}

		if ref.ClientEmail == "" {
			for i := range clients {
				if clients[i].Enable {
					sources = append(sources, ProxySource{Inbound: inbound, Client: &clients[i]})
				}
			}
			continue
		}

		for i := range clients {
			if clients[i].Email == ref.ClientEmail && clients[i].Enable {
				sources = append(sources, ProxySource{Inbound: inbound, Client: &clients[i]})
				break
			}
		}
	}

	return sources, nil
}
