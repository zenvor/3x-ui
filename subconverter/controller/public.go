package controller

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	submodel "github.com/mhsanaei/3x-ui/v3/subconverter/model"
	"github.com/mhsanaei/3x-ui/v3/subconverter/service"
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
	usage      *service.SubscriptionUsageService
	settings   *service.SettingsService
	accessLogs *service.AccessLogService
}

// NewPublicController wires both /feed routes onto the root engine and
// returns the handle.
func NewPublicController(engine *gin.Engine) *PublicController {
	p := &PublicController{
		subSvc:     service.NewSubscriptionService(),
		resolver:   service.NewInboundResolver(),
		ipBindings: service.NewIPBindingService(),
		usage:      service.NewSubscriptionUsageService(),
		settings:   service.NewSettingsService(),
		accessLogs: service.NewAccessLogService(),
	}
	engine.GET("/feed/:token", p.full)
	engine.GET("/feed/:token/nodes", p.provider)
	return p
}

const yamlContentType = "application/x-yaml; charset=utf-8"
const subscriptionTokenLength = 32

// full serves /feed/:token. It binds the requesting IP on first hit and
// rejects with 403 once the subscription's MaxIps quota is full.
func (p *PublicController) full(c *gin.Context) {
	if p.rejectDisallowedUserAgent(c, service.AccessEndpointFull) {
		return
	}
	sub := p.lookup(c)
	if sub == nil {
		return // status already written
	}
	if p.rejectDisabledSubscription(c, sub, service.AccessEndpointFull) {
		return
	}
	ip := remoteIP(c)
	if ip == "" {
		p.recordAccess(sub, service.AccessEndpointFull, c, "", http.StatusBadRequest, service.AccessResultIPMissing)
		c.String(http.StatusBadRequest, "cannot determine client ip")
		return
	}
	if err := p.ipBindings.Enforce(sub.Id, sub.MaxIps, ip); err != nil {
		if errors.Is(err, service.ErrIPLimitExceeded) {
			p.recordAccess(sub, service.AccessEndpointFull, c, ip, http.StatusForbidden, service.AccessResultIPLimitExceeded)
			c.String(http.StatusForbidden, "ip limit exceeded")
			return
		}
		p.recordAccess(sub, service.AccessEndpointFull, c, ip, http.StatusInternalServerError, service.AccessResultInternalError)
		c.String(http.StatusInternalServerError, "ip enforcement failed")
		return
	}
	_ = p.usage.RecordCompleted(sub.Id, ip, c.Request.UserAgent())
	p.recordAccess(sub, service.AccessEndpointFull, c, ip, http.StatusOK, service.AccessResultSuccess)
	c.Header("Content-Type", yamlContentType)
	c.String(http.StatusOK, service.RenderMihomoYAML(apiDomain(c), sub.Token))
}

// provider serves /feed/:token/nodes. It does not bind new IPs but it does
// respect the quota: when the IP is unknown and quota is exhausted, the
// response is HTTP 403 with an empty proxy list (matches sublinker).
func (p *PublicController) provider(c *gin.Context) {
	if p.rejectDisallowedUserAgent(c, service.AccessEndpointNodes) {
		return
	}
	sub := p.lookup(c)
	if sub == nil {
		return
	}
	if p.rejectDisabledSubscription(c, sub, service.AccessEndpointNodes) {
		return
	}
	ip := remoteIP(c)
	if ip == "" {
		p.recordAccess(sub, service.AccessEndpointNodes, c, "", http.StatusBadRequest, service.AccessResultIPMissing)
		c.String(http.StatusBadRequest, "cannot determine client ip")
		return
	}

	status := http.StatusOK
	result := service.AccessResultSuccess
	if err := p.ipBindings.CheckOnly(sub.Id, sub.MaxIps, ip); err != nil {
		if !errors.Is(err, service.ErrIPLimitExceeded) {
			p.recordAccess(sub, service.AccessEndpointNodes, c, ip, http.StatusInternalServerError, service.AccessResultInternalError)
			c.String(http.StatusInternalServerError, "ip check failed")
			return
		}
		status = http.StatusForbidden
		result = service.AccessResultIPLimitExceeded
	}

	var proxies []service.MihomoProxy
	if status == http.StatusOK {
		proxies = p.resolveProxies(sub, c)
	}

	body, err := service.RenderMihomoProviderYAML(proxies)
	if err != nil {
		p.recordAccess(sub, service.AccessEndpointNodes, c, ip, http.StatusInternalServerError, service.AccessResultInternalError)
		c.String(http.StatusInternalServerError, "render failed")
		return
	}
	p.recordAccess(sub, service.AccessEndpointNodes, c, ip, status, result)
	c.Header("Content-Type", yamlContentType)
	c.String(status, body)
}

// lookup loads the subscription, writes a 404/500 response if it cannot, and
// returns nil to signal the caller should stop. Returns the subscription on
// success.
func (p *PublicController) lookup(c *gin.Context) *submodel.Subscription {
	token := c.Param("token")
	if !validSubscriptionToken(token) {
		c.String(http.StatusNotFound, "subscription not found")
		return nil
	}
	sub, err := p.subSvc.FindByToken(token)
	if err != nil {
		c.String(http.StatusInternalServerError, "lookup failed")
		return nil
	}
	if sub == nil {
		c.String(http.StatusNotFound, "subscription not found")
		return nil
	}
	return sub
}

func (p *PublicController) rejectDisabledSubscription(c *gin.Context, sub *submodel.Subscription, endpoint string) bool {
	if sub.Enabled {
		return false
	}
	p.recordAccess(sub, endpoint, c, remoteIP(c), http.StatusNotFound, service.AccessResultSubscriptionDisabled)
	c.String(http.StatusNotFound, "subscription not found")
	return true
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

func (p *PublicController) rejectDisallowedUserAgent(c *gin.Context, endpoint string) bool {
	settings, err := p.settings.Get()
	if err != nil {
		logger.Warning("subconverter UA policy load failed:", err)
		c.String(http.StatusInternalServerError, "user-agent policy failed")
		return true
	}
	if service.IsUserAgentAllowed(c.Request.UserAgent(), settings) {
		return false
	}

	status := settings.UARejectStatus
	if status != http.StatusNotFound {
		status = http.StatusForbidden
	}
	logger.Warningf("subconverter feed UA rejected: ip=%s ua=%q status=%d",
		remoteIP(c), c.Request.UserAgent(), status)
	p.recordKnownTokenAccess(c, endpoint, status, service.AccessResultUARejected)
	if status == http.StatusNotFound {
		c.String(http.StatusNotFound, "subscription not found")
	} else {
		c.String(http.StatusForbidden, "forbidden")
	}
	return true
}

func (p *PublicController) recordKnownTokenAccess(c *gin.Context, endpoint string, status int, result string) {
	token := c.Param("token")
	if !validSubscriptionToken(token) {
		return
	}
	sub, err := p.subSvc.FindByToken(token)
	if err != nil {
		logger.Warning("subconverter access log token lookup failed:", err)
		return
	}
	if sub == nil {
		return
	}
	p.recordAccess(sub, endpoint, c, remoteIP(c), status, result)
}

func validSubscriptionToken(token string) bool {
	if len(token) != subscriptionTokenLength {
		return false
	}
	for _, r := range token {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func (p *PublicController) recordAccess(sub *submodel.Subscription, endpoint string, c *gin.Context, ip string, status int, result string) {
	if sub == nil {
		return
	}
	if err := p.accessLogs.Record(service.AccessLogInput{
		SubscriptionId: sub.Id,
		Endpoint:       endpoint,
		Ip:             ip,
		UserAgent:      c.Request.UserAgent(),
		StatusCode:     status,
		Result:         result,
	}); err != nil {
		logger.Warning("subconverter access log write failed:", err)
	}
}

// apiDomain returns "scheme://host" for the public-facing request. Used to
// fill the __API_DOMAIN__ placeholder in the Mihomo template.
func apiDomain(c *gin.Context) string {
	scheme := ""
	host := ""
	if isTrustedForwardedRequest(c) {
		scheme = forwardedProto(c.GetHeader("X-Forwarded-Proto"))
		host = forwardedHost(c.GetHeader("X-Forwarded-Host"))
	}
	if scheme == "" && c.Request.TLS != nil {
		scheme = "https"
	}
	if scheme == "" {
		scheme = "http"
	}
	if host == "" {
		host = c.Request.Host
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

// requestHostOnly returns just the host portion (no scheme, no port) — used
// as the fallback proxy address when an inbound is bound to 0.0.0.0.
func requestHostOnly(c *gin.Context) string {
	host := ""
	if isTrustedForwardedRequest(c) {
		host = forwardedHost(c.GetHeader("X-Forwarded-Host"))
	}
	if host == "" {
		host = c.Request.Host
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func forwardedProto(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.Index(value, ","); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	switch strings.ToLower(value) {
	case "http", "https":
		return strings.ToLower(value)
	default:
		return ""
	}
}

func forwardedHost(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.Index(value, ","); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	return value
}
