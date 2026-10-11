package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

func validateRemoteTemplateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return errors.New("use an HTTPS URL without credentials or a fragment")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.Contains(host, "%") {
		return errors.New("template URL must use a public host")
	}
	if ip, err := netip.ParseAddr(host); err == nil && !publicTemplateIP(ip) {
		return errors.New("template URL must use a public address")
	}
	if port := u.Port(); port != "" {
		if _, err := net.LookupPort("tcp", port); err != nil {
			return errors.New("invalid HTTPS port")
		}
	}
	return nil
}

// Global unicast includes private and reserved ranges, so exclude them too.
var reservedTemplateNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

func publicTemplateIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, network := range reservedTemplateNetworks {
		if network.Contains(ip) {
			return false
		}
	}
	// Reject IPv4-compatible/tunnel addresses outside ordinary public IPv6.
	return ip.Is4() || netip.MustParsePrefix("2000::/3").Contains(ip)
}

// Resolve once and dial only the validated IPs, preventing DNS rebinding. The
// hostname remains in the HTTP request and TLS certificate verification.
func dialPublicTemplate(ctx context.Context, network, address string, lookup func(context.Context, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("template host has no addresses")
	}
	for _, ip := range ips {
		if !publicTemplateIP(ip) {
			return nil, errors.New("template host resolves to a non-public address")
		}
	}
	var last error
	for _, ip := range ips {
		conn, err := dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}

// Kept as a factory so service tests can simulate HTTPS responses without an
// external server. Production always uses the guarded direct transport.
var remoteTemplateClient = func() *http.Client {
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   templateFetchTimeout,
		ResponseHeaderTimeout: templateFetchTimeout,
		DisableKeepAlives:     true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: templateFetchTimeout}
			return dialPublicTemplate(ctx, network, address,
				func(ctx context.Context, host string) ([]netip.Addr, error) {
					return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
				}, dialer.DialContext)
		},
	}
	return &http.Client{
		Timeout:   templateFetchTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return errors.New("too many template redirects")
			}
			if err := validateRemoteTemplateURL(req.URL.String()); err != nil {
				return fmt.Errorf("template redirect rejected: %w", err)
			}
			// Conditional validators belong to the original source, not a redirect target.
			req.Header.Del("If-None-Match")
			return nil
		},
	}
}
