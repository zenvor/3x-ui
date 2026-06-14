package service

import (
	"testing"

	xmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInboundOptionCapabilities(t *testing.T) {
	cases := []struct {
		name                string
		inbound             *xmodel.Inbound
		wantSubconverter    bool
		wantCDNTLSCandidate bool
	}{
		{
			name:                "tcp reality exports directly",
			inbound:             vlessInbound("tcp reality", "", 443, realityStream()),
			wantSubconverter:    true,
			wantCDNTLSCandidate: false,
		},
		{
			name:                "xhttp reality exports directly",
			inbound:             vlessInbound("xhttp reality", "", 443, xhttpRealityStream()),
			wantSubconverter:    true,
			wantCDNTLSCandidate: false,
		},
		{
			name:                "bare xhttp requires CDN TLS",
			inbound:             vlessInbound("xhttp none", "", 80, xhttpNoneStream()),
			wantSubconverter:    false,
			wantCDNTLSCandidate: true,
		},
		{
			name: "ws tls is not supported by Mihomo subconverter",
			inbound: vlessInbound("ws tls", "", 443, `{
				"network":"ws",
				"security":"tls"
			}`),
			wantSubconverter:    false,
			wantCDNTLSCandidate: false,
		},
		{
			name: "disabled reality is not selectable",
			inbound: func() *xmodel.Inbound {
				in := vlessInbound("disabled", "", 443, realityStream())
				in.Enable = false
				return in
			}(),
			wantSubconverter:    false,
			wantCDNTLSCandidate: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsVlessSupportedInbound(tc.inbound, ProxyOptions{}); got != tc.wantSubconverter {
				t.Fatalf("subconverterCapable = %v, want %v", got, tc.wantSubconverter)
			}
			if got := isCDNTLSCandidate(tc.inbound); got != tc.wantCDNTLSCandidate {
				t.Fatalf("cdnTlsCapable = %v, want %v", got, tc.wantCDNTLSCandidate)
			}
		})
	}
}
