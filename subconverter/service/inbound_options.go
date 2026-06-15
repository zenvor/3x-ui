package service

import (
	xmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xservice "github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// InboundOption is the picker shape used by the subconverter admin UI.
type InboundOption struct {
	Id                  int    `json:"id"`
	Remark              string `json:"remark"`
	Tag                 string `json:"tag"`
	Protocol            string `json:"protocol"`
	Port                int    `json:"port"`
	CdnTlsCapable       bool   `json:"cdnTlsCapable"`
	SubconverterCapable bool   `json:"subconverterCapable"`
}

type InboundOptionService struct {
	inboundSvc *xservice.InboundService
}

func NewInboundOptionService() *InboundOptionService {
	return &InboundOptionService{inboundSvc: &xservice.InboundService{}}
}

func (s *InboundOptionService) List(userID int) ([]InboundOption, error) {
	inbounds, err := s.inboundSvc.GetInbounds(userID)
	if err != nil {
		return nil, err
	}
	out := make([]InboundOption, 0, len(inbounds))
	for _, inbound := range inbounds {
		if inbound == nil {
			continue
		}
		out = append(out, InboundOption{
			Id:                  inbound.Id,
			Remark:              inbound.Remark,
			Tag:                 inbound.Tag,
			Protocol:            string(inbound.Protocol),
			Port:                inbound.Port,
			CdnTlsCapable:       isCDNTLSCandidate(inbound),
			SubconverterCapable: IsVlessSupportedInbound(inbound, ProxyOptions{}),
		})
	}
	return out, nil
}

func isCDNTLSCandidate(inbound *xmodel.Inbound) bool {
	transport, reality, _, err := parseVlessSupported(inbound, ProxyOptions{
		CDNTLS: &CDNTLSOptions{Enabled: true, Server: "cdn.example.com"},
	})
	return err == nil && canApplyCDNTLSOverlay(transport, reality)
}
