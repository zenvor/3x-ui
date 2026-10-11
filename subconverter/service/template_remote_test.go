package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"testing"
)

func TestRemoteTemplateURLAndIPRestrictions(t *testing.T) {
	for _, raw := range []string{"http://example.com/a", "https://user:password@example.com/a", "https://example.com/a#fragment", "https://localhost/a", "https://a.localhost/a", "https://127.0.0.1/a", "https://10.0.0.1/a", "https://169.254.169.254/a", "https://[::1]/a", "https://[::ffff:127.0.0.1]/a", "https://100.64.0.1/a", "https://192.0.2.1/a", "https://[2001:db8::1]/a", "https://[64:ff9b::a00:1]/a", "https://example.com:wrong/a"} {
		if err := validateRemoteTemplateURL(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"https://raw.githubusercontent.com/user/repo/main/mihomo.yaml", "https://example.com:8443/a?version=2", "https://1.1.1.1/a", "https://[2606:4700:4700::1111]/a"} {
		if err := validateRemoteTemplateURL(raw); err != nil {
			t.Errorf("rejected %s: %v", raw, err)
		}
	}
	for _, raw := range []string{"0.0.0.0", "172.16.1.1", "192.168.1.1", "224.0.0.1", "255.255.255.255", "198.18.1.1", "::", "fc00::1", "fe80::1", "2002:a00:1::", "::ffff:10.0.0.1"} {
		if publicTemplateIP(netip.MustParseAddr(raw)) {
			t.Errorf("accepted IP %s", raw)
		}
	}
}

func TestRemoteTemplateNumericPortBounds(t *testing.T) {
	for _, port := range []string{"0", "65536", "999999999999999999999", "-1", "https"} {
		if err := validateRemoteTemplateURL("https://example.com:" + port + "/a"); err == nil {
			t.Errorf("accepted port %s", port)
		}
	}
	for _, port := range []string{"1", "443", "8443", "65535"} {
		if err := validateRemoteTemplateURL("https://example.com:" + port + "/a"); err != nil {
			t.Errorf("rejected port %s: %v", port, err)
		}
	}
}

func TestRemoteTemplateClientBlocksUnsafeRedirectBeforeRequest(t *testing.T) {
	for _, target := range []string{"http://templates.example/a", "https://127.0.0.1/a", "https://169.254.169.254/a", "https://[::ffff:10.0.0.1]/a", "https://user:password@example.com/a"} {
		t.Run(target, func(t *testing.T) {
			client := remoteTemplateClient()
			requests := 0
			client.Transport = templateRoundTripper(func(r *http.Request) (*http.Response, error) {
				requests++
				response := templateResponse(http.StatusFound, "", "")
				response.Request = r
				response.Header.Set("Location", target)
				return response, nil
			})
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://templates.example/start", nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(req)
			if response != nil {
				response.Body.Close()
			}
			if err == nil || requests != 1 {
				t.Fatalf("unsafe redirect reached transport: %d requests, %v", requests, err)
			}
		})
	}
}

func TestRemoteTemplateDialRejectsUnsafeDNSBeforeAnyConnection(t *testing.T) {
	for _, addresses := range [][]string{nil, {"127.0.0.1"}, {"169.254.169.254"}, {"::ffff:10.0.0.1"}, {"1.1.1.1", "fc00::1"}} {
		lookup := func(context.Context, string) ([]netip.Addr, error) {
			var result []netip.Addr
			for _, address := range addresses {
				result = append(result, netip.MustParseAddr(address))
			}
			return result, nil
		}
		dialed := false
		dial := func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, errors.New("unexpected network access")
		}
		if _, err := dialPublicTemplate(t.Context(), "tcp", "templates.example:443", lookup, dial); err == nil || dialed {
			t.Fatalf("unsafe DNS reached network: %v, %v", addresses, err)
		}
	}
}

func TestRemoteTemplateDialPinsPublicDNSAndRejectsMixedAnswers(t *testing.T) {
	resolved := 0
	var dialed []string
	lookup := func(_ context.Context, host string) ([]netip.Addr, error) {
		resolved++
		if host != "templates.example" {
			t.Fatalf("unexpected hostname %s", host)
		}
		return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("8.8.8.8")}, nil
	}
	dial := func(_ context.Context, _, address string) (net.Conn, error) {
		dialed = append(dialed, address)
		return nil, errors.New("simulated failure")
	}
	if _, err := dialPublicTemplate(context.Background(), "tcp", "templates.example:443", lookup, dial); err == nil {
		t.Fatal("expected dial failure")
	}
	if resolved != 1 || len(dialed) != 2 || dialed[0] != "1.1.1.1:443" || dialed[1] != "8.8.8.8:443" {
		t.Fatalf("DNS not pinned: %d %v", resolved, dialed)
	}
	dialed = nil
	mixed := func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("10.0.0.1")}, nil
	}
	if _, err := dialPublicTemplate(context.Background(), "tcp", "templates.example:443", mixed, dial); err == nil || len(dialed) != 0 {
		t.Fatal("mixed DNS answer reached network")
	}
}

func TestRemoteTemplateRedirectsRemainHTTPSAndPublic(t *testing.T) {
	client := remoteTemplateClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.DialContext == nil || !transport.DisableKeepAlives {
		t.Fatal("unguarded transport")
	}
	for _, raw := range []string{"http://example.com/a", "https://10.0.0.1/a", "https://user@example.com/a"} {
		u, _ := url.Parse(raw)
		if err := client.CheckRedirect(&http.Request{URL: u, Header: make(http.Header)}, []*http.Request{{}}); err == nil {
			t.Errorf("accepted redirect %s", raw)
		}
	}
	u, _ := url.Parse("https://other.example/a")
	req := &http.Request{URL: u, Header: http.Header{"If-None-Match": []string{`"old"`}}}
	if err := client.CheckRedirect(req, []*http.Request{{}}); err != nil || req.Header.Get("If-None-Match") != "" {
		t.Fatal("valid redirect failed or forwarded ETag")
	}
	if err := client.CheckRedirect(req, make([]*http.Request, 4)); err == nil {
		t.Fatal("unbounded redirects")
	}
}
