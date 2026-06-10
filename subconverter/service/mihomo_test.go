package service

import (
	"testing"

	xmodel "github.com/mhsanaei/3x-ui/v3/database/model"
)

func vlessInbound(remark, listen string, port int, streamSettings string) *xmodel.Inbound {
	return &xmodel.Inbound{
		Id:             1,
		Remark:         remark,
		Enable:         true,
		Listen:         listen,
		Port:           port,
		Protocol:       xmodel.VLESS,
		StreamSettings: streamSettings,
		Settings:       `{"clients":[{"id":"uuid-1","email":"alice@x"}]}`,
	}
}

func client(uuid, email, flow string) *xmodel.Client {
	return &xmodel.Client{ID: uuid, Email: email, Flow: flow, Enable: true}
}

func realityStream() string {
	return `{
		"network":"tcp",
		"security":"reality",
		"realitySettings":{
			"serverNames":["www.cloudflare.com","www.amazon.com"],
			"shortIds":["abcd1234"],
			"settings":{"publicKey":"pubkey-xyz","fingerprint":"chrome"}
		}
	}`
}

func TestConvertVlessTCPReality(t *testing.T) {
	in := vlessInbound("home", "203.0.113.5", 443, realityStream())
	cl := client("uuid-1", "alice@x", "xtls-rprx-vision")

	proxy, err := ConvertInboundToProxy(in, cl, "fallback.example.com")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.Type != "vless" || proxy.Server != "203.0.113.5" || proxy.Port != 443 {
		t.Fatalf("base fields wrong: %+v", proxy)
	}
	if !proxy.TLS {
		t.Error("TLS should be true for reality")
	}
	if proxy.Servername != "www.cloudflare.com" {
		t.Errorf("servername = %q, want first of serverNames", proxy.Servername)
	}
	if proxy.Flow != "xtls-rprx-vision" {
		t.Errorf("flow = %q, want xtls-rprx-vision (tcp+reality)", proxy.Flow)
	}
	if proxy.RealityOpts == nil || proxy.RealityOpts.PublicKey != "pubkey-xyz" || proxy.RealityOpts.ShortId != "abcd1234" {
		t.Errorf("reality-opts wrong: %+v", proxy.RealityOpts)
	}
	if proxy.ClientFingerprint != "chrome" {
		t.Errorf("fingerprint = %q, want chrome", proxy.ClientFingerprint)
	}
	if proxy.Network != "" {
		t.Errorf("network should be omitted for tcp, got %q", proxy.Network)
	}
}

func TestConvertVlessWSTLSRejected(t *testing.T) {
	stream := `{
		"network":"ws",
		"security":"tls",
		"wsSettings":{"path":"/ws","host":"cdn.example.com"},
		"tlsSettings":{
			"serverName":"cdn.example.com",
			"alpn":["h2","http/1.1"],
			"settings":{"fingerprint":"firefox"}
		}
	}`
	in := vlessInbound("cdn", "10.0.0.5", 8443, stream)
	cl := client("uuid-1", "bob@x", "")

	if _, err := ConvertInboundToProxy(in, cl, "panel.example.com"); err != ErrUnsupportedInbound {
		t.Fatalf("err = %v, want ErrUnsupportedInbound", err)
	}
}

func TestConvertVlessGRPCRejected(t *testing.T) {
	stream := `{
		"network":"grpc",
		"security":"tls",
		"grpcSettings":{"serviceName":"trojan-grpc"},
		"tlsSettings":{"serverName":"a.example.com"}
	}`
	in := vlessInbound("grpc-node", "1.2.3.4", 443, stream)
	cl := client("uuid-1", "x", "")

	if _, err := ConvertInboundToProxy(in, cl, "fb"); err != ErrUnsupportedInbound {
		t.Fatalf("err = %v, want ErrUnsupportedInbound", err)
	}
}

func TestConvertListenWildcardUsesFallback(t *testing.T) {
	for _, listen := range []string{"", "0.0.0.0", "::", "::0"} {
		in := vlessInbound("r", listen, 443, realityStream())
		cl := client("u", "e", "")
		proxy, err := ConvertInboundToProxy(in, cl, "panel.example.com")
		if err != nil {
			t.Fatalf("listen=%q: %v", listen, err)
		}
		if proxy.Server != "panel.example.com" {
			t.Errorf("listen=%q: server = %q, want fallback", listen, proxy.Server)
		}
		if !proxy.TLS {
			t.Errorf("listen=%q: TLS should be true for reality", listen)
		}
	}
}

func TestConvertNonVlessProtocolRejected(t *testing.T) {
	in := &xmodel.Inbound{
		Id:       1,
		Protocol: xmodel.VMESS,
		Listen:   "1.2.3.4",
		Port:     443,
		Enable:   true,
	}
	cl := client("u", "e", "")
	if _, err := ConvertInboundToProxy(in, cl, "fb"); err != ErrUnsupportedProtocol {
		t.Fatalf("err = %v, want ErrUnsupportedProtocol", err)
	}
}

func TestConvertNameFallback(t *testing.T) {
	cases := []struct {
		remark, email string
		want          string
	}{
		{"home", "alice@x", "home-alice@x"},
		{"home", "", "home"},
		{"", "alice@x", "alice@x"},
		{"", "", "inbound-1"},
	}
	for _, c := range cases {
		in := vlessInbound(c.remark, "1.2.3.4", 443, realityStream())
		cl := client("u", c.email, "")
		proxy, err := ConvertInboundToProxy(in, cl, "fb")
		if err != nil {
			t.Fatalf("remark=%q email=%q: %v", c.remark, c.email, err)
		}
		if proxy.Name != c.want {
			t.Errorf("remark=%q email=%q: name = %q, want %q", c.remark, c.email, proxy.Name, c.want)
		}
	}
}

func TestConvertTLSSecurityRejected(t *testing.T) {
	stream := `{"network":"tcp","security":"tls","tlsSettings":{}}`
	in := vlessInbound("r", "host.example.com", 443, stream)
	cl := client("u", "e", "")
	if _, err := ConvertInboundToProxy(in, cl, "fb"); err != ErrUnsupportedInbound {
		t.Fatalf("err = %v, want ErrUnsupportedInbound", err)
	}
}

func TestConvertTCPNoneSecurityRejected(t *testing.T) {
	stream := `{"network":"tcp","security":"none"}`
	in := vlessInbound("r", "host.example.com", 443, stream)
	cl := client("u", "e", "")
	if _, err := ConvertInboundToProxy(in, cl, "fb"); err != ErrUnsupportedInbound {
		t.Fatalf("err = %v, want ErrUnsupportedInbound", err)
	}
}

func TestConvertExternalProxyRules(t *testing.T) {
	cases := []struct {
		name    string
		stream  string
		wantErr bool
	}{
		{
			name: "empty external proxy allowed",
			stream: `{
				"network":"tcp",
				"security":"reality",
				"externalProxy":[],
				"realitySettings":{
					"serverNames":["www.cloudflare.com"],
					"shortIds":["abcd1234"],
					"settings":{"publicKey":"pubkey-xyz","fingerprint":"chrome"}
				}
			}`,
		},
		{
			name: "non-empty external proxy rejected",
			stream: `{
				"network":"tcp",
				"security":"reality",
				"externalProxy":[{"dest":"cdn.example.com","port":443}],
				"realitySettings":{
					"serverNames":["www.cloudflare.com"],
					"shortIds":["abcd1234"],
					"settings":{"publicKey":"pubkey-xyz","fingerprint":"chrome"}
				}
			}`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := vlessInbound("r", "host.example.com", 443, tc.stream)
			cl := client("u", "e", "")
			_, err := ConvertInboundToProxy(in, cl, "fb")
			if tc.wantErr && err != ErrUnsupportedInbound {
				t.Fatalf("err = %v, want ErrUnsupportedInbound", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("convert: %v", err)
			}
		})
	}
}

func TestConvertTCPHTTPHeaderRejected(t *testing.T) {
	stream := `{
		"network":"tcp",
		"security":"reality",
		"tcpSettings":{"header":{"type":"http"}},
		"realitySettings":{
			"serverNames":["www.cloudflare.com"],
			"shortIds":["abcd1234"],
			"settings":{"publicKey":"pubkey-xyz","fingerprint":"chrome"}
		}
	}`
	in := vlessInbound("r", "host.example.com", 443, stream)
	cl := client("u", "e", "")
	if _, err := ConvertInboundToProxy(in, cl, "fb"); err != ErrUnsupportedInbound {
		t.Fatalf("err = %v, want ErrUnsupportedInbound", err)
	}
}
