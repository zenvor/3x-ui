package service

import (
	"testing"

	xmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

func convertForTest(inbound *xmodel.Inbound, client *xmodel.Client, hostFallback string) (*MihomoProxy, error) {
	return ConvertInboundToProxy(inbound, client, hostFallback, ProxyOptions{})
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

func xhttpRealityStream() string {
	return `{
		"network":"xhttp",
		"xhttpSettings":{
			"path":"/abc123",
			"host":"",
			"mode":"auto",
			"xPaddingBytes":"100-1000",
			"scMaxBufferedPosts":30,
			"scStreamUpServerSecs":"20-80"
		},
		"security":"reality",
		"realitySettings":{
			"serverNames":["www.intel.com"],
			"shortIds":["08","cdc5f11159bf97"],
			"settings":{"publicKey":"pubkey-xhttp","fingerprint":"chrome","spiderX":"/"}
		}
	}`
}

func xhttpNoneStream() string {
	return `{
		"network":"xhttp",
		"xhttpSettings":{
			"path":"/cdn-path",
			"host":"",
			"mode":"auto",
			"xPaddingBytes":"100-1000"
		},
		"security":"none"
	}`
}

func xhttpTLSStream() string {
	return `{
		"network":"xhttp",
		"xhttpSettings":{
			"path":"/tls-path",
			"host":"cdn.example.com",
			"mode":"auto",
			"xPaddingBytes":"100-1000"
		},
		"security":"tls",
		"tlsSettings":{
			"serverName":"cdn.example.com",
			"alpn":["h2","http/1.1"],
			"certificates":[
				{"certificateFile":"/root/cert/fullchain.pem","keyFile":"/root/cert/private.key"}
			],
			"settings":{"fingerprint":"firefox"}
		}
	}`
}

func TestConvertVlessTCPReality(t *testing.T) {
	in := vlessInbound("home", "203.0.113.5", 443, realityStream())
	cl := client("uuid-1", "alice@x", "xtls-rprx-vision")

	proxy, err := convertForTest(in, cl, "fallback.example.com")
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

func TestConvertVlessXHTTPReality(t *testing.T) {
	in := vlessInbound("xhttp", "", 443, xhttpRealityStream())
	cl := client("uuid-1", "alice@x", "xtls-rprx-vision")

	proxy, err := convertForTest(in, cl, "panel.example.com")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.Network != "xhttp" {
		t.Fatalf("network = %q, want xhttp", proxy.Network)
	}
	if proxy.Server != "panel.example.com" {
		t.Errorf("server = %q, want fallback", proxy.Server)
	}
	if !proxy.TLS {
		t.Error("TLS should be true for xhttp reality")
	}
	if proxy.Flow != "" {
		t.Errorf("flow should be omitted for xhttp, got %q", proxy.Flow)
	}
	if proxy.Servername != "www.intel.com" {
		t.Errorf("servername = %q, want www.intel.com", proxy.Servername)
	}
	if proxy.RealityOpts == nil || proxy.RealityOpts.PublicKey != "pubkey-xhttp" || proxy.RealityOpts.ShortId != "08" {
		t.Errorf("reality-opts wrong: %+v", proxy.RealityOpts)
	}
	if proxy.XHTTPOpts == nil {
		t.Fatal("xhttp-opts missing")
	}
	if proxy.XHTTPOpts.Path != "/abc123" || proxy.XHTTPOpts.Host != "" || proxy.XHTTPOpts.Mode != "auto" {
		t.Errorf("xhttp-opts = %+v, want path /abc123 and mode auto only", proxy.XHTTPOpts)
	}
	if proxy.XHTTPOpts.XPaddingBytes != "100-1000" {
		t.Errorf("x-padding-bytes = %q, want 100-1000", proxy.XHTTPOpts.XPaddingBytes)
	}
}

func TestConvertVlessXHTTPRealityHostFromHeaders(t *testing.T) {
	stream := `{
		"network":"xhttp",
		"xhttpSettings":{
			"path":"/xh",
			"headers":{"Host":["cdn.example.com"]},
			"mode":"packet-up"
		},
		"security":"reality",
		"realitySettings":{
			"serverNames":["www.intel.com"],
			"shortIds":["08"],
			"settings":{"publicKey":"pubkey-xhttp","fingerprint":"chrome"}
		}
	}`
	in := vlessInbound("xhttp", "203.0.113.10", 443, stream)
	cl := client("uuid-1", "alice@x", "")

	proxy, err := convertForTest(in, cl, "panel.example.com")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.XHTTPOpts == nil || proxy.XHTTPOpts.Host != "cdn.example.com" {
		t.Fatalf("host should fall back to headers.Host, got %+v", proxy.XHTTPOpts)
	}
}

func TestConvertVlessXHTTPRealityAdvancedClientOptions(t *testing.T) {
	stream := `{
		"network":"xhttp",
		"xhttpSettings":{
			"path":"/xh",
			"mode":"packet-up",
			"headers":{"X-Test":"ok","Host":"cdn.example.com"},
			"xPaddingBytes":"500-1500",
			"xPaddingObfsMode":true,
			"xPaddingKey":"pad-key",
			"xPaddingHeader":"X-Pad",
			"xPaddingPlacement":"query",
			"xPaddingMethod":"random",
			"uplinkHTTPMethod":"PUT",
			"sessionPlacement":"header",
			"sessionKey":"sid",
			"seqPlacement":"query",
			"seqKey":"seq",
			"uplinkDataPlacement":"body",
			"uplinkDataKey":"u",
			"uplinkChunkSize":8192,
			"scMaxEachPostBytes":2000000,
			"scMinPostsIntervalMs":60,
			"noGRPCHeader":true,
			"scMaxBufferedPosts":30,
			"scStreamUpServerSecs":"20-80",
			"noSSEHeader":true,
			"serverMaxHeaderBytes":4096
		},
		"security":"reality",
		"realitySettings":{
			"serverNames":["www.intel.com"],
			"shortIds":["08"],
			"settings":{"publicKey":"pubkey-xhttp","fingerprint":"chrome"}
		}
	}`
	in := vlessInbound("xhttp", "203.0.113.10", 443, stream)
	cl := client("uuid-1", "alice@x", "")

	proxy, err := convertForTest(in, cl, "panel.example.com")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	opts := proxy.XHTTPOpts
	if opts == nil {
		t.Fatal("xhttp-opts missing")
	}
	if opts.XPaddingBytes != "500-1500" || !opts.XPaddingObfsMode || opts.XPaddingKey != "pad-key" {
		t.Fatalf("padding opts wrong: %+v", opts)
	}
	if opts.UplinkHTTPMethod != "PUT" || opts.UplinkChunkSize != "8192" || !opts.NoGRPCHeader {
		t.Fatalf("uplink opts wrong: %+v", opts)
	}
	if opts.ScMaxEachPostBytes != "2000000" || opts.ScMinPostsInterval != "60" {
		t.Fatalf("sc opts wrong: %+v", opts)
	}
	if opts.Headers == nil || opts.Headers["X-Test"] != "ok" {
		t.Fatalf("headers wrong: %+v", opts.Headers)
	}
	if _, ok := opts.Headers["Host"]; ok {
		t.Fatalf("headers should not duplicate Host: %+v", opts.Headers)
	}
}

func TestConvertVlessXHTTPRealityFiltersRedundantDefaults(t *testing.T) {
	stream := `{
		"network":"xhttp",
		"xhttpSettings":{
			"path":"/xh",
			"mode":"auto",
			"scMaxEachPostBytes":"1000000",
			"scMinPostsIntervalMs":"30",
			"uplinkChunkSize":"0"
		},
		"security":"reality",
		"realitySettings":{
			"serverNames":["www.intel.com"],
			"shortIds":["08"],
			"settings":{"publicKey":"pubkey-xhttp","fingerprint":"chrome"}
		}
	}`
	in := vlessInbound("xhttp", "203.0.113.10", 443, stream)
	cl := client("uuid-1", "alice@x", "")

	proxy, err := convertForTest(in, cl, "panel.example.com")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.XHTTPOpts.ScMaxEachPostBytes != "" || proxy.XHTTPOpts.ScMinPostsInterval != "" || proxy.XHTTPOpts.UplinkChunkSize != "" {
		t.Fatalf("redundant defaults should be omitted: %+v", proxy.XHTTPOpts)
	}
}

func TestConvertVlessXHTTPNoneWithCDNTLS(t *testing.T) {
	in := vlessInbound("cdn", "0.0.0.0", 80, xhttpNoneStream())
	cl := client("uuid-1", "alice@x", "")

	proxy, err := ConvertInboundToProxy(in, cl, "panel.example.com", ProxyOptions{
		CDNTLS: &CDNTLSOptions{
			Enabled:    true,
			Server:     "203.0.113.20",
			Port:       443,
			Servername: "cdn.example.com",
		},
	})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.Server != "203.0.113.20" || proxy.Port != 443 {
		t.Fatalf("cdn endpoint wrong: %+v", proxy)
	}
	if !proxy.TLS || proxy.Servername != "cdn.example.com" || proxy.ClientFingerprint != "chrome" {
		t.Fatalf("tls fields wrong: %+v", proxy)
	}
	if len(proxy.ALPN) != 1 || proxy.ALPN[0] != "h2" {
		t.Fatalf("alpn = %#v, want h2", proxy.ALPN)
	}
	if proxy.RealityOpts != nil {
		t.Fatalf("reality-opts should be omitted for CDN TLS: %+v", proxy.RealityOpts)
	}
	if proxy.Network != "xhttp" || proxy.XHTTPOpts == nil {
		t.Fatalf("xhttp fields missing: %+v", proxy)
	}
	if proxy.XHTTPOpts.Path != "/cdn-path" || proxy.XHTTPOpts.Host != "" || proxy.XHTTPOpts.Mode != "auto" {
		t.Fatalf("xhttp-opts wrong: %+v", proxy.XHTTPOpts)
	}
	if proxy.XHTTPOpts.XPaddingBytes != "100-1000" {
		t.Fatalf("x-padding-bytes = %q, want 100-1000", proxy.XHTTPOpts.XPaddingBytes)
	}
}

func TestConvertVlessXHTTPNoneWithCDNTLSDefaults(t *testing.T) {
	in := vlessInbound("cdn", "", 6666, xhttpNoneStream())
	cl := client("uuid-1", "alice@x", "")

	proxy, err := ConvertInboundToProxy(in, cl, "panel.example.com", ProxyOptions{
		CDNTLS: &CDNTLSOptions{
			Enabled: true,
			Server:  "cdn.example.com",
		},
	})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.Server != "cdn.example.com" || proxy.Port != 443 {
		t.Fatalf("default endpoint wrong: %+v", proxy)
	}
	if proxy.Servername != "cdn.example.com" || proxy.XHTTPOpts.Host != "" {
		t.Fatalf("default sni/host wrong: servername=%q opts=%+v", proxy.Servername, proxy.XHTTPOpts)
	}
	if len(proxy.ALPN) != 1 || proxy.ALPN[0] != "h2" {
		t.Fatalf("default alpn = %#v, want h2", proxy.ALPN)
	}
}

func TestConvertVlessXHTTPTLS(t *testing.T) {
	in := vlessInbound("xhttp tls", "", 6666, xhttpTLSStream())
	cl := client("uuid-1", "alice@x", "xtls-rprx-vision")

	proxy, err := convertForTest(in, cl, "panel.example.com")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.Server != "panel.example.com" || proxy.Port != 6666 {
		t.Fatalf("native TLS endpoint wrong: %+v", proxy)
	}
	if !proxy.TLS || proxy.Servername != "cdn.example.com" || proxy.ClientFingerprint != "firefox" {
		t.Fatalf("tls fields wrong: %+v", proxy)
	}
	if len(proxy.ALPN) != 2 || proxy.ALPN[0] != "h2" || proxy.ALPN[1] != "http/1.1" {
		t.Fatalf("alpn = %#v, want h2,http/1.1", proxy.ALPN)
	}
	if proxy.RealityOpts != nil {
		t.Fatalf("reality-opts should be omitted for native TLS: %+v", proxy.RealityOpts)
	}
	if proxy.Flow != "" {
		t.Fatalf("flow should be omitted for xhttp tls, got %q", proxy.Flow)
	}
	if proxy.Network != "xhttp" || proxy.XHTTPOpts == nil {
		t.Fatalf("xhttp fields missing: %+v", proxy)
	}
	if proxy.XHTTPOpts.Path != "/tls-path" || proxy.XHTTPOpts.Host != "cdn.example.com" || proxy.XHTTPOpts.Mode != "auto" {
		t.Fatalf("xhttp-opts wrong: %+v", proxy.XHTTPOpts)
	}
	if proxy.XHTTPOpts.XPaddingBytes != "100-1000" {
		t.Fatalf("x-padding-bytes = %q, want 100-1000", proxy.XHTTPOpts.XPaddingBytes)
	}
}

func TestConvertVlessXHTTPTLSWithCDNTLS(t *testing.T) {
	in := vlessInbound("xhttp tls", "", 6666, xhttpTLSStream())
	cl := client("uuid-1", "alice@x", "xtls-rprx-vision")

	proxy, err := ConvertInboundToProxy(in, cl, "panel.example.com", ProxyOptions{
		CDNTLS: &CDNTLSOptions{
			Enabled:    true,
			Server:     "203.0.113.20",
			Port:       443,
			Servername: "edge.example.com",
		},
	})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.Server != "203.0.113.20" || proxy.Port != 443 {
		t.Fatalf("cdn endpoint wrong: %+v", proxy)
	}
	if !proxy.TLS || proxy.Servername != "edge.example.com" || proxy.ClientFingerprint != "chrome" {
		t.Fatalf("tls fields wrong: %+v", proxy)
	}
	if len(proxy.ALPN) != 1 || proxy.ALPN[0] != "h2" {
		t.Fatalf("alpn = %#v, want h2", proxy.ALPN)
	}
	if proxy.RealityOpts != nil {
		t.Fatalf("reality-opts should be omitted for CDN TLS: %+v", proxy.RealityOpts)
	}
	if proxy.Flow != "" {
		t.Fatalf("flow should be omitted for xhttp tls, got %q", proxy.Flow)
	}
	if proxy.Network != "xhttp" || proxy.XHTTPOpts == nil {
		t.Fatalf("xhttp fields missing: %+v", proxy)
	}
	if proxy.XHTTPOpts.Path != "/tls-path" || proxy.XHTTPOpts.Host != "cdn.example.com" || proxy.XHTTPOpts.Mode != "auto" {
		t.Fatalf("xhttp-opts wrong: %+v", proxy.XHTTPOpts)
	}
	if proxy.XHTTPOpts.XPaddingBytes != "100-1000" {
		t.Fatalf("x-padding-bytes = %q, want 100-1000", proxy.XHTTPOpts.XPaddingBytes)
	}
}

func TestConvertVlessXHTTPRealityIgnoresCDNTLSOverlay(t *testing.T) {
	in := vlessInbound("xhttp", "", 443, xhttpRealityStream())
	cl := client("uuid-1", "alice@x", "")

	proxy, err := ConvertInboundToProxy(in, cl, "panel.example.com", ProxyOptions{
		CDNTLS: &CDNTLSOptions{
			Enabled:    true,
			Server:     "cdn.example.com",
			Port:       443,
			Servername: "cdn.example.com",
		},
	})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.Server != "panel.example.com" || proxy.Servername != "www.intel.com" {
		t.Fatalf("reality endpoint should not be overwritten by CDN TLS options: %+v", proxy)
	}
	if proxy.RealityOpts == nil || proxy.RealityOpts.PublicKey != "pubkey-xhttp" {
		t.Fatalf("reality-opts should be preserved: %+v", proxy.RealityOpts)
	}
	if len(proxy.ALPN) != 0 {
		t.Fatalf("reality conversion should not inherit CDN ALPN: %#v", proxy.ALPN)
	}
	if proxy.XHTTPOpts == nil || proxy.XHTTPOpts.Host != "" {
		t.Fatalf("xhttp host should not be overwritten by CDN TLS options: %+v", proxy.XHTTPOpts)
	}
}

func TestConvertVlessXHTTPNoneWithoutCDNTLS(t *testing.T) {
	in := vlessInbound("cdn", "", 80, xhttpNoneStream())
	cl := client("uuid-1", "alice@x", "")

	proxy, err := convertForTest(in, cl, "panel.example.com")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if proxy.Server != "panel.example.com" || proxy.Port != 80 {
		t.Fatalf("endpoint wrong: %+v", proxy)
	}
	if proxy.TLS || proxy.Servername != "" || proxy.ClientFingerprint != "" || len(proxy.ALPN) != 0 {
		t.Fatalf("plain xhttp should not emit TLS fields: %+v", proxy)
	}
	if proxy.RealityOpts != nil {
		t.Fatalf("reality-opts should be omitted for plain xhttp: %+v", proxy.RealityOpts)
	}
	if proxy.Network != "xhttp" || proxy.XHTTPOpts == nil {
		t.Fatalf("xhttp fields missing: %+v", proxy)
	}
	if proxy.XHTTPOpts.Path != "/cdn-path" || proxy.XHTTPOpts.Host != "" || proxy.XHTTPOpts.Mode != "auto" {
		t.Fatalf("xhttp-opts wrong: %+v", proxy.XHTTPOpts)
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

	if _, err := convertForTest(in, cl, "panel.example.com"); err != ErrUnsupportedInbound {
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

	if _, err := convertForTest(in, cl, "fb"); err != ErrUnsupportedInbound {
		t.Fatalf("err = %v, want ErrUnsupportedInbound", err)
	}
}

func TestConvertListenWildcardUsesFallback(t *testing.T) {
	for _, listen := range []string{"", "0.0.0.0", "::", "::0"} {
		in := vlessInbound("r", listen, 443, realityStream())
		cl := client("u", "e", "")
		proxy, err := convertForTest(in, cl, "panel.example.com")
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
	if _, err := convertForTest(in, cl, "fb"); err != ErrUnsupportedProtocol {
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
		proxy, err := convertForTest(in, cl, "fb")
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
	if _, err := convertForTest(in, cl, "fb"); err != ErrUnsupportedInbound {
		t.Fatalf("err = %v, want ErrUnsupportedInbound", err)
	}
}

func TestConvertTCPNoneSecurityRejected(t *testing.T) {
	stream := `{"network":"tcp","security":"none"}`
	in := vlessInbound("r", "host.example.com", 443, stream)
	cl := client("u", "e", "")
	if _, err := convertForTest(in, cl, "fb"); err != ErrUnsupportedInbound {
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
			_, err := convertForTest(in, cl, "fb")
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
	if _, err := convertForTest(in, cl, "fb"); err != ErrUnsupportedInbound {
		t.Fatalf("err = %v, want ErrUnsupportedInbound", err)
	}
}
