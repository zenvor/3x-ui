package controller

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	submodel "github.com/mhsanaei/3x-ui/v2/subconverter/model"
	"github.com/mhsanaei/3x-ui/v2/subconverter/service"
)

// PublicController serves the unauthenticated /feed/* endpoints that Mihomo
// clients hit directly.
//
// Routes:
//
//	GET /feed/:token         full Mihomo YAML (binds IP, denies on quota)
//	GET /feed/:token/nodes   Mihomo proxy-provider YAML (read-only IP check)
type PublicController struct {
	subSvc     *service.SubscriptionService
	resolver   *service.InboundResolver
	ipBindings *service.IPBindingService
}

// NewPublicController wires both /feed routes onto the root engine and
// returns the handle.
func NewPublicController(engine *gin.Engine) *PublicController {
	p := &PublicController{
		subSvc:     service.NewSubscriptionService(),
		resolver:   service.NewInboundResolver(),
		ipBindings: service.NewIPBindingService(),
	}
	engine.GET("/feed/:token", p.full)
	engine.GET("/feed/:token/nodes", p.provider)
	return p
}

const yamlContentType = "application/x-yaml; charset=utf-8"

// full serves /feed/:token. It binds the requesting IP on first hit and
// rejects with 403 once the subscription's MaxIps quota is full.
func (p *PublicController) full(c *gin.Context) {
	sub := p.lookup(c)
	if sub == nil {
		return // status already written
	}
	ip := remoteIP(c)
	if ip == "" {
		c.String(http.StatusBadRequest, "cannot determine client ip")
		return
	}
	if err := p.ipBindings.Enforce(sub.Id, sub.MaxIps, ip); err != nil {
		if errors.Is(err, service.ErrIPLimitExceeded) {
			c.String(http.StatusForbidden, "ip limit exceeded")
			return
		}
		c.String(http.StatusInternalServerError, "ip enforcement failed")
		return
	}
	c.Header("Content-Type", yamlContentType)
	c.String(http.StatusOK, service.RenderMihomoYAML(apiDomain(c), sub.Token))
}

// provider serves /feed/:token/nodes. It does not bind new IPs but it does
// respect the quota: when the IP is unknown and quota is exhausted, the
// response is HTTP 403 with an empty proxy list (matches sublinker).
func (p *PublicController) provider(c *gin.Context) {
	sub := p.lookup(c)
	if sub == nil {
		return
	}
	ip := remoteIP(c)
	if ip == "" {
		c.String(http.StatusBadRequest, "cannot determine client ip")
		return
	}

	status := http.StatusOK
	if err := p.ipBindings.CheckOnly(sub.Id, sub.MaxIps, ip); err != nil {
		if !errors.Is(err, service.ErrIPLimitExceeded) {
			c.String(http.StatusInternalServerError, "ip check failed")
			return
		}
		status = http.StatusForbidden
	}

	var proxies []service.MihomoProxy
	if status == http.StatusOK {
		proxies = p.resolveProxies(sub, c)
	}

	body, err := service.RenderProviderYAML(proxies)
	if err != nil {
		c.String(http.StatusInternalServerError, "render failed")
		return
	}
	c.Header("Content-Type", yamlContentType)
	c.String(status, body)
}

// lookup loads the subscription, writes a 404/500 response if it cannot, and
// returns nil to signal the caller should stop. Returns the subscription on
// success.
func (p *PublicController) lookup(c *gin.Context) *submodel.Subscription {
	token := c.Param("token")
	sub, err := p.subSvc.FindByToken(token)
	if err != nil {
		c.String(http.StatusInternalServerError, "lookup failed")
		return nil
	}
	if sub == nil || !sub.Enabled {
		c.String(http.StatusNotFound, "subscription not found")
		return nil
	}
	return sub
}

func (p *PublicController) resolveProxies(sub *submodel.Subscription, c *gin.Context) []service.MihomoProxy {
	sources, err := p.resolver.Resolve(sub.Inbounds)
	if err != nil || len(sources) == 0 {
		return nil
	}
	host := requestHostOnly(c)
	proxies := make([]service.MihomoProxy, 0, len(sources))
	for _, src := range sources {
		proxy, convErr := service.ConvertInboundToProxy(src.Inbound, src.Client, host)
		if convErr != nil || proxy == nil {
			continue
		}
		proxies = append(proxies, *proxy)
	}
	return proxies
}

// remoteIP picks the most reliable client IP from request headers, mirroring
// 3X-UI's web/controller.getRemoteIp.
func remoteIP(c *gin.Context) string {
	if v := c.GetHeader("X-Real-IP"); v != "" {
		return v
	}
	if v := c.GetHeader("X-Forwarded-For"); v != "" {
		parts := strings.Split(v, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

// apiDomain returns "scheme://host" for the public-facing request. Used to
// fill the __API_DOMAIN__ placeholder in the Mihomo template.
func apiDomain(c *gin.Context) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

// requestHostOnly returns just the host portion (no scheme, no port) — used
// as the fallback proxy address when an inbound is bound to 0.0.0.0.
func requestHostOnly(c *gin.Context) string {
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
