package service

import (
	"strings"

	xdatabase "github.com/mhsanaei/3x-ui/v3/internal/database"
	xmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xservice "github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// InboundOption is the picker shape used by the subconverter admin UI.
type InboundOption struct {
	Id                  int                   `json:"id"`
	Remark              string                `json:"remark"`
	Tag                 string                `json:"tag"`
	Protocol            string                `json:"protocol"`
	Port                int                   `json:"port"`
	CdnTlsCapable       bool                  `json:"cdnTlsCapable"`
	SubconverterCapable bool                  `json:"subconverterCapable"`
	Clients             []InboundOptionClient `json:"clients"`
}

type InboundOptionClient struct {
	Email      string `json:"email"`
	Enable     bool   `json:"enable"`
	HasID      bool   `json:"hasId"`
	TotalGB    int64  `json:"totalGB"`
	ExpiryTime int64  `json:"expiryTime"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
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
			Clients:             optionClients(inbound, s.inboundSvc),
		})
	}
	return out, nil
}

func optionClients(inbound *xmodel.Inbound, inboundSvc *xservice.InboundService) []InboundOptionClient {
	clients, err := inboundSvc.GetClients(inbound)
	if err != nil || len(clients) == 0 {
		return []InboundOptionClient{}
	}
	emails := inboundClientEmails(clients)
	trafficByEmail := optionClientTrafficByEmail(emails)
	recordByEmail := optionClientRecordByEmail(emails)

	seen := make(map[string]bool, len(emails))
	out := make([]InboundOptionClient, 0, len(emails))
	for _, client := range clients {
		email := strings.TrimSpace(client.Email)
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true

		row := InboundOptionClient{
			Email:      email,
			Enable:     client.Enable,
			HasID:      strings.TrimSpace(client.ID) != "",
			TotalGB:    client.TotalGB,
			ExpiryTime: client.ExpiryTime,
		}
		record, hasRecord := recordByEmail[email]
		if hasRecord {
			row.TotalGB = record.TotalGB
			row.ExpiryTime = record.ExpiryTime
		}
		if traffic, ok := trafficByEmail[email]; ok {
			row.Up = traffic.Up
			row.Down = traffic.Down
			if !hasRecord && row.TotalGB == 0 {
				row.TotalGB = traffic.Total
			}
			if !hasRecord && row.ExpiryTime == 0 {
				row.ExpiryTime = traffic.ExpiryTime
			}
		}
		out = append(out, row)
	}
	return out
}

func inboundClientEmails(clients []xmodel.Client) []string {
	seen := make(map[string]bool, len(clients))
	emails := make([]string, 0, len(clients))
	for _, client := range clients {
		email := strings.TrimSpace(client.Email)
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		emails = append(emails, email)
	}
	return emails
}

func optionClientTrafficByEmail(emails []string) map[string]xray.ClientTraffic {
	out := make(map[string]xray.ClientTraffic, len(emails))
	if len(emails) == 0 {
		return out
	}
	var rows []xray.ClientTraffic
	if err := xdatabase.GetDB().Where("email IN ?", emails).Find(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		email := strings.TrimSpace(row.Email)
		if email == "" {
			continue
		}
		out[email] = row
	}
	return out
}

func optionClientRecordByEmail(emails []string) map[string]xmodel.ClientRecord {
	out := make(map[string]xmodel.ClientRecord, len(emails))
	if len(emails) == 0 {
		return out
	}
	var rows []xmodel.ClientRecord
	if err := xdatabase.GetDB().Where("email IN ?", emails).Find(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		email := strings.TrimSpace(row.Email)
		if email == "" {
			continue
		}
		out[email] = row
	}
	return out
}

func isCDNTLSCandidate(inbound *xmodel.Inbound) bool {
	transport, reality, _, err := parseVlessSupported(inbound, ProxyOptions{
		CDNTLS: &CDNTLSOptions{Enabled: true, Server: "cdn.example.com"},
	})
	return err == nil && canApplyCDNTLSOverlay(transport, reality)
}
