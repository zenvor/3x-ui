package panel

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
)

func TestSponsorSourcesDoNotFollowRedirects(t *testing.T) {
	setupSponsorServer(t, `{"sponsors":[]}`)
	var internalHits atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits.Add(1)
		_, _ = w.Write([]byte(`{"sponsors":[]}`))
	}))
	defer internal.Close()
	for _, code := range []int{301, 302, 303, 307, 308} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			for _, path := range []string{"/sponsors.json", "/media/acme.png"} {
				source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, internal.URL+path, code)
				}))
				_, err := fetchLimited(source.URL+path, maxSponsorsBytes)
				source.Close()
				if err == nil || !strings.Contains(err.Error(), "returned status") {
					t.Fatalf("redirect response = %v, want rejected non-200", err)
				}
			}
		})
	}
	if hits := internalHits.Load(); hits != 0 {
		t.Fatalf("redirect reached internal service %d times", hits)
	}
}

func TestSponsorDirectDialBlocksPrivateAddresses(t *testing.T) {
	setupSponsorServer(t, `{"sponsors":[]}`)
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	client := newSponsorHTTPClient()
	defer client.CloseIdleConnections()
	for _, address := range []string{target.URL, strings.Replace(target.URL, "127.0.0.1", "localhost", 1), "http://10.0.0.1/sponsors.json", "http://169.254.169.254/sponsors.json"} {
		response, err := client.Get(address)
		if response != nil {
			_ = response.Body.Close()
		}
		if !errors.Is(err, netsafe.ErrPrivateAddressBlocked) {
			t.Fatalf("request to %s: error = %v, want private address rejection", address, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("guarded dial reached a private service")
	}
}

func TestSponsorRedirectProtectionPreservesConfiguredProxy(t *testing.T) {
	var proxyHits, internalHits atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { internalHits.Add(1) }))
	defer internal.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		http.Redirect(w, r, internal.URL, http.StatusFound)
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	client := secureSponsorHTTPClient(&http.Client{Transport: transport, Timeout: time.Second})
	defer client.CloseIdleConnections()
	if client.Transport != transport {
		t.Fatal("security policy replaced the configured proxy transport")
	}
	response, err := client.Get("http://sponsors.example.test/3X/sponsors.json")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusFound || proxyHits.Load() != 1 || internalHits.Load() != 0 {
		t.Fatalf("status=%d proxy=%d internal=%d, want redirect rejected before the next request", response.StatusCode, proxyHits.Load(), internalHits.Load())
	}
}
